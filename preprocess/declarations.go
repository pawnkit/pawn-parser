package preprocess

import (
	"slices"
	"strconv"
	"strings"

	"github.com/pawnkit/pawn-parser/token"
)

// declarationTracker retains the part of PawnCC's live symbol table that is
// observable through the preprocessor's `defined` operator. PawnCC parses and
// preprocesses incrementally, so declarations earlier in the token stream are
// visible to later conditional directives.
type declarationTracker struct {
	maxNameLength int
	packedStrings bool
	functions     map[string]struct{}
	declaredFuncs map[string]struct{}
	usedBefore    map[string]struct{}
	globals       map[string]struct{}
	constants     map[string][]declarationToken
	resolved      map[string]string
	arrays        map[string]arraySize
	enumSizes     map[string]int64
	enumFields    map[string]int64
	activeEnum    *enumDeclaration
	scopes        []map[string]struct{}
	header        []declarationToken
	parenDepth    int
	bracketDepth  int
	pendingParams []string
	pendingFunc   bool
	needsReparse  bool
}

type arraySize struct {
	dimensions []int64
	enumIndex  string
}

type enumDeclaration struct {
	name         string
	entry        []declarationToken
	parenDepth   int
	bracketDepth int
	nextValue    int64
}

type declarationToken struct {
	kind token.Kind
	text string
}

func newDeclarationTracker(maxNameLength int, seedFunctions map[string]struct{}, resolved map[string]string, packedStrings bool) *declarationTracker {
	tracker := &declarationTracker{
		maxNameLength: maxNameLength,
		packedStrings: packedStrings,
		functions:     make(map[string]struct{}, len(seedFunctions)),
		declaredFuncs: make(map[string]struct{}),
		usedBefore:    make(map[string]struct{}),
		globals:       make(map[string]struct{}),
		constants:     make(map[string][]declarationToken),
		resolved:      make(map[string]string, len(resolved)),
		arrays:        make(map[string]arraySize),
		enumSizes:     make(map[string]int64),
		enumFields:    make(map[string]int64),
	}
	for name := range seedFunctions {
		tracker.functions[tracker.name(name)] = struct{}{}
	}
	for name, value := range resolved {
		tracker.resolved[tracker.name(name)] = value
	}
	return tracker
}

func (t *declarationTracker) name(name string) string {
	if t != nil && t.maxNameLength > 0 && len(name) > t.maxNameLength {
		return name[:t.maxNameLength]
	}
	return name
}

func (t *declarationTracker) defined(name string) bool {
	if t == nil {
		return false
	}
	name = t.name(name)
	for _, v := range slices.Backward(t.scopes) {
		if _, ok := v[name]; ok {
			return true
		}
	}
	if _, ok := t.functions[name]; ok {
		return true
	}
	_, ok := t.globals[name]
	return ok
}

func (t *declarationTracker) constant(name string) ([]ptok, bool) {
	if t == nil {
		return nil, false
	}
	items, ok := t.constants[t.name(name)]
	if !ok {
		return nil, false
	}
	if value, resolved := t.resolved[t.name(name)]; resolved {
		return tokenizeBody(value), true
	}
	tokens := make([]ptok, len(items))
	for index, item := range items {
		tokens[index] = ptok{Token: token.Token{Kind: item.kind}, text: item.text}
	}
	return tokens, true
}

//nolint:gocyclo // Declaration tracking handles several independent token states.
func (t *declarationTracker) observe(item ptok) {
	if t == nil || item.Kind == token.EOF {
		return
	}
	entry := declarationToken{kind: item.Kind, text: item.text}
	if item.Kind == token.LBrace {
		t.beginEnum()
	} else if t.activeEnum != nil {
		t.observeEnumToken(entry)
	}
	if item.Kind == token.LParen {
		t.observeCall()
	}

	//nolint:exhaustive // Only braces change declaration state here.
	switch item.Kind {
	case token.LBrace:
		t.finishHeader(true, false)
		scope := make(map[string]struct{}, len(t.pendingParams))
		if t.pendingFunc {
			for _, name := range t.pendingParams {
				scope[t.name(name)] = struct{}{}
			}
		}
		t.scopes = append(t.scopes, scope)
		t.pendingParams = nil
		t.pendingFunc = false
		t.header = nil
		return
	case token.RBrace:
		t.finishHeader(false, false)
		if len(t.scopes) != 0 {
			t.scopes = t.scopes[:len(t.scopes)-1]
		}
		t.header = nil
		return
	}

	t.header = append(t.header, entry)
	//nolint:exhaustive // Only delimiter kinds change nesting state here.
	switch item.Kind {
	case token.LParen:
		t.parenDepth++
	case token.RParen:
		if t.parenDepth > 0 {
			t.parenDepth--
		}
	case token.LBracket:
		t.bracketDepth++
	case token.RBracket:
		if t.bracketDepth > 0 {
			t.bracketDepth--
		}
	case token.Semicolon:
		if t.parenDepth == 0 && t.bracketDepth == 0 {
			t.finishHeader(false, false)
		}
	}

	if strings.ContainsAny(item.trailing, "\r\n") && t.parenDepth == 0 && t.bracketDepth == 0 &&
		!headerContains(t.header, token.KwEnum) {
		t.finishHeader(false, true)
	}
}

func (t *declarationTracker) finishHeader(body, mayHaveBody bool) {
	if len(t.header) == 0 {
		return
	}
	t.pendingFunc = false
	t.pendingParams = nil
	//nolint:nestif // Scope recovery handles malformed in-progress declarations.
	if len(t.scopes) == 0 {
		if name, params, tagged, ok := functionDeclaration(t.header); ok {
			name = t.name(name)
			t.functions[t.name(name)] = struct{}{}
			if _, used := t.usedBefore[name]; used && tagged {
				t.needsReparse = true
			}
			t.declaredFuncs[name] = struct{}{}
			t.pendingFunc = body || mayHaveBody
			if body || mayHaveBody {
				t.pendingParams = params
			}
		}
		for name, expression := range constantDeclarations(t.header) {
			t.constants[t.name(name)] = expression
		}
	}
	t.recordArrayDeclarations(t.header)
	for _, name := range variableDeclarations(t.header) {
		t.declareValue(name)
	}
	t.header = nil
	t.parenDepth = 0
	t.bracketDepth = 0
}

func (t *declarationTracker) beginEnum() {
	if t == nil || t.activeEnum != nil {
		return
	}
	keyword := -1
	for index, item := range t.header {
		if item.kind == token.KwEnum {
			keyword = index
			break
		}
	}
	if keyword < 0 {
		return
	}
	name := ""
	for _, item := range t.header[keyword+1:] {
		if item.kind == token.Identifier {
			name = t.name(item.text)
			break
		}
	}
	t.activeEnum = &enumDeclaration{name: name}
}

func (t *declarationTracker) observeEnumToken(item declarationToken) {
	state := t.activeEnum
	if state == nil {
		return
	}
	//nolint:exhaustive // Enum tracking only handles delimiters and separators.
	switch item.kind {
	case token.LParen:
		state.parenDepth++
	case token.RParen:
		if state.parenDepth > 0 {
			state.parenDepth--
		}
	case token.LBracket:
		state.bracketDepth++
	case token.RBracket:
		if state.bracketDepth > 0 {
			state.bracketDepth--
		}
	case token.Comma:
		if state.parenDepth == 0 && state.bracketDepth == 0 {
			t.finishEnumEntry()
			return
		}
	case token.RBrace:
		if state.parenDepth == 0 && state.bracketDepth == 0 {
			t.finishEnumEntry()
			if state.name != "" {
				t.enumSizes[state.name] = state.nextValue
			}
			t.activeEnum = nil
			return
		}
	}
	state.entry = append(state.entry, item)
}

func (t *declarationTracker) finishEnumEntry() {
	state := t.activeEnum
	if state == nil || len(state.entry) == 0 {
		return
	}
	nameIndex := -1
	for index, item := range state.entry {
		if item.kind == token.Identifier {
			nameIndex = index
			break
		}
	}
	if nameIndex < 0 {
		state.entry = nil
		return
	}
	name := t.name(state.entry[nameIndex].text)
	if assign := declarationTokenIndex(state.entry, token.Assign); assign >= 0 {
		if value, ok := t.declarationInteger(state.entry[assign+1:]); ok {
			state.nextValue = value
		}
	}
	span := int64(1)
	if open := declarationTokenIndex(state.entry[nameIndex+1:], token.LBracket); open >= 0 {
		open += nameIndex + 1
		if closing := matchingDeclarationBracket(state.entry, open); closing > open {
			if value, ok := t.declarationInteger(state.entry[open+1 : closing]); ok && value > 0 {
				span = value
			}
		}
	}
	t.enumFields[name] = span
	t.constants[name] = []declarationToken{{kind: token.IntLiteral, text: strconv.FormatInt(state.nextValue, 10)}}
	state.nextValue += span
	state.entry = nil
}

func (t *declarationTracker) observeCall() {
	if t == nil || len(t.header) == 0 {
		return
	}
	previous := t.header[len(t.header)-1]
	if !macroNameText(previous.text) || nonCallKeyword(previous.kind) {
		return
	}
	if len(t.scopes) == 0 && !declarationStartsValue(t.header) && !headerContains(t.header, token.Assign) {
		return
	}
	name := t.name(previous.text)
	t.functions[name] = struct{}{}
	if _, declared := t.declaredFuncs[name]; !declared {
		t.usedBefore[name] = struct{}{}
	}
}

func nonCallKeyword(kind token.Kind) bool {
	switch kind {
	case token.KwIf, token.KwFor, token.KwWhile, token.KwSwitch, token.KwSizeof,
		token.KwTagof, token.KwDefined, token.KwReturn:
		return true
	default:
		return false
	}
}

func headerContains(items []declarationToken, kind token.Kind) bool {
	for _, item := range items {
		if item.kind == kind {
			return true
		}
	}
	return false
}

func (t *declarationTracker) declareValue(name string) {
	name = t.name(name)
	if name == "" {
		return
	}
	if len(t.scopes) == 0 {
		t.globals[name] = struct{}{}
		return
	}
	t.scopes[len(t.scopes)-1][name] = struct{}{}
}

func functionDeclaration(items []declarationToken) (string, []string, bool, bool) {
	if len(items) == 0 || declarationStartsValue(items) {
		return "", nil, false, false
	}
	depth := 0
	open := -1
	for index, item := range items {
		//nolint:exhaustive // Function headers only inspect declaration delimiters.
		switch item.kind {
		case token.Assign:
			if depth == 0 {
				return "", nil, false, false
			}
		case token.LParen:
			if depth == 0 {
				open = index
				break
			}
			depth++
		}
		if open >= 0 {
			break
		}
	}
	if open <= 0 {
		return "", nil, false, false
	}
	name := ""
	nameIndex := -1
	for index := open - 1; index >= 0; index-- {
		if macroNameText(items[index].text) && !declarationModifier(items[index].kind) {
			name = items[index].text
			nameIndex = index
			break
		}
	}
	if name == "" {
		return "", nil, false, false
	}
	tagged := nameIndex >= 2 && items[nameIndex-1].kind == token.Colon && items[nameIndex-2].text != "_"
	closing := matchingParen(items, open)
	if closing < 0 {
		return name, nil, tagged, true
	}
	return name, parameterNames(items[open+1 : closing]), tagged, true
}

func declarationStartsValue(items []declarationToken) bool {
	for _, item := range items {
		switch item.kind {
		case token.KwNew, token.KwConst, token.KwEnum, token.KwReturn,
			token.KwIf, token.KwElse, token.KwFor, token.KwWhile, token.KwSwitch:
			return true
		case token.KwStatic, token.KwStock, token.KwPublic:
			continue
		default:
			return false
		}
	}
	return false
}

func declarationModifier(kind token.Kind) bool {
	switch kind {
	case token.KwNative, token.KwForward, token.KwPublic, token.KwStock, token.KwStatic, token.KwConst:
		return true
	default:
		return false
	}
}

func matchingParen(items []declarationToken, open int) int {
	depth := 0
	for index := open; index < len(items); index++ {
		//nolint:exhaustive // Matching only tracks parentheses.
		switch items[index].kind {
		case token.LParen:
			depth++
		case token.RParen:
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func parameterNames(items []declarationToken) []string {
	var names []string
	start := 0
	depth := 0
	for index := 0; index <= len(items); index++ {
		atEnd := index == len(items)
		if !atEnd {
			//nolint:exhaustive // Parameter parsing only tracks nested delimiters.
			switch items[index].kind {
			case token.LParen, token.LBracket, token.LBrace:
				depth++
			case token.RParen, token.RBracket, token.RBrace:
				if depth > 0 {
					depth--
				}
			}
		}
		if atEnd || items[index].kind == token.Comma && depth == 0 {
			if name := declaratorName(items[start:index]); name != "" {
				names = append(names, name)
			}
			start = index + 1
		}
	}
	return names
}

func variableDeclarations(items []declarationToken) []string {
	keyword := -1
	for index, item := range items {
		if item.kind == token.KwNew || item.kind == token.KwConst {
			keyword = index
			break
		}
	}
	if keyword < 0 {
		return nil
	}
	return parameterNames(items[keyword+1:])
}

//nolint:gocyclo // Declaration segmentation keeps nested delimiters in one pass.
func constantDeclarations(items []declarationToken) map[string][]declarationToken {
	keyword := -1
	for index, item := range items {
		if item.kind == token.KwConst {
			keyword = index
			break
		}
	}
	if keyword < 0 {
		return nil
	}

	result := make(map[string][]declarationToken)
	start := keyword + 1
	depth := 0
	for index := start; index <= len(items); index++ {
		atEnd := index == len(items)
		if !atEnd {
			//nolint:exhaustive // Declaration parsing only tracks nested delimiters.
			switch items[index].kind {
			case token.LParen, token.LBracket, token.LBrace:
				depth++
			case token.RParen, token.RBracket, token.RBrace:
				if depth > 0 {
					depth--
				}
			}
		}
		if !atEnd && (items[index].kind != token.Comma || depth != 0) && items[index].kind != token.Semicolon {
			continue
		}
		segment := items[start:index]
		assign := -1
		segmentDepth := 0
		for position, item := range segment {
			//nolint:exhaustive // Declaration parsing only tracks nesting and assignment.
			switch item.kind {
			case token.LParen, token.LBracket, token.LBrace:
				segmentDepth++
			case token.RParen, token.RBracket, token.RBrace:
				if segmentDepth > 0 {
					segmentDepth--
				}
			case token.Assign:
				if segmentDepth == 0 {
					assign = position
				}
			}
			if assign >= 0 {
				break
			}
		}
		if assign >= 0 {
			if name := declaratorName(segment[:assign]); name != "" {
				result[name] = append([]declarationToken(nil), segment[assign+1:]...)
			}
		}
		start = index + 1
		if atEnd || items[index].kind == token.Semicolon {
			break
		}
	}
	return result
}

func declaratorName(items []declarationToken) string {
	end := len(items)
	depth := 0
	for index, item := range items {
		//nolint:exhaustive // Declarator parsing only tracks nested delimiters.
		switch item.kind {
		case token.LParen, token.LBracket, token.LBrace:
			depth++
		case token.RParen, token.RBracket, token.RBrace:
			if depth > 0 {
				depth--
			}
		case token.Assign:
			if depth == 0 {
				end = index
				goto foundEnd
			}
		}
	}
foundEnd:
	for index := end - 1; index >= 0; index-- {
		if macroNameText(items[index].text) && !declarationModifier(items[index].kind) {
			return items[index].text
		}
	}
	return ""
}

//nolint:gocyclo // Array declarators need one pass over mixed dimensions.
func (t *declarationTracker) recordArrayDeclarations(items []declarationToken) {
	if t == nil || !declarationTokenContains(items, token.LBracket) {
		return
	}
	for _, segment := range splitDeclarationSegments(items) {
		assign := topLevelDeclarationToken(segment, token.Assign)
		declarator := segment
		var initializer []declarationToken
		if assign >= 0 {
			declarator = segment[:assign]
			initializer = segment[assign+1:]
		}
		nameIndex := -1
		for index, item := range declarator {
			if item.kind == token.LBracket {
				break
			}
			if item.kind == token.Identifier && !declarationModifier(item.kind) {
				nameIndex = index
			}
		}
		if nameIndex < 0 {
			continue
		}
		var dimensions []int64
		enumIndex := ""
		for index := nameIndex + 1; index < len(declarator); index++ {
			if declarator[index].kind != token.LBracket {
				continue
			}
			closing := matchingDeclarationBracket(declarator, index)
			if closing < 0 {
				break
			}
			expression := declarator[index+1 : closing]
			value, ok := t.declarationInteger(expression)
			if !ok {
				value = 0
			}
			if len(dimensions) == 0 && len(expression) == 1 && expression[0].kind == token.Identifier {
				if _, exists := t.enumSizes[t.name(expression[0].text)]; exists {
					enumIndex = t.name(expression[0].text)
				}
			}
			dimensions = append(dimensions, value)
			index = closing
		}
		if len(dimensions) == 0 {
			continue
		}
		if dimensions[0] == 0 {
			if inferred, ok := t.inferredArraySize(initializer); ok {
				dimensions[0] = inferred
			}
		}
		if dimensions[0] <= 0 {
			continue
		}
		t.arrays[t.name(declarator[nameIndex].text)] = arraySize{dimensions: dimensions, enumIndex: enumIndex}
	}
}

func (t *declarationTracker) inferredArraySize(initializer []declarationToken) (int64, bool) {
	for _, item := range initializer {
		if item.kind != token.StringLiteral && item.kind != token.PackedString {
			continue
		}
		characters := pawnStringCharacters(item.text)
		packed := item.kind == token.PackedString || t.packedStrings
		if packed {
			return int64((characters + 1 + 3) / 4), true
		}
		return int64(characters + 1), true
	}
	return 0, false
}

func pawnStringCharacters(text string) int {
	start := strings.IndexByte(text, '"')
	end := strings.LastIndexByte(text, '"')
	if start < 0 || end <= start {
		return 0
	}
	characters := 0
	for index := start + 1; index < end; {
		if text[index] != '\\' || index+1 >= end {
			characters++
			index++
			continue
		}
		index++
		if index < end && text[index] >= '0' && text[index] <= '9' {
			for index < end && text[index] >= '0' && text[index] <= '9' {
				index++
			}
			if index < end && text[index] == ';' {
				index++
			}
		} else {
			index++
		}
		characters++
	}
	return characters
}

func (t *declarationTracker) declarationInteger(items []declarationToken) (int64, bool) {
	for len(items) >= 2 && items[0].kind == token.LParen && items[len(items)-1].kind == token.RParen {
		items = items[1 : len(items)-1]
	}
	if len(items) == 0 {
		return 0, false
	}
	sign := int64(1)
	//nolint:exhaustive // Integer declarations only accept supported literal forms.
	switch items[0].kind {
	case token.Minus:
		sign = -1
		items = items[1:]
	case token.Plus:
		items = items[1:]
	}
	if len(items) != 1 {
		return 0, false
	}
	//nolint:exhaustive // Integer declarations only accept supported literal forms.
	switch items[0].kind {
	case token.IntLiteral:
		value, err := strconv.ParseInt(strings.ReplaceAll(items[0].text, "_", ""), 0, 64)
		return sign * value, err == nil
	case token.Identifier:
		name := t.name(items[0].text)
		if value, ok := t.enumSizes[name]; ok {
			return sign * value, true
		}
		if constant, ok := t.constants[name]; ok {
			value, resolved := t.declarationInteger(constant)
			return sign * value, resolved
		}
	}
	return 0, false
}

func (t *declarationTracker) sizeOf(name string, selectors []string) (int64, bool) {
	if t == nil {
		return 0, false
	}
	array, ok := t.arrays[t.name(name)]
	if !ok || len(array.dimensions) == 0 {
		return 0, false
	}
	if len(selectors) == 0 {
		return array.dimensions[0], true
	}
	if array.enumIndex != "" {
		if span, exists := t.enumFields[t.name(selectors[0])]; exists {
			return span, true
		}
	}
	if len(selectors) < len(array.dimensions) {
		return array.dimensions[len(selectors)], true
	}
	return 1, true
}

func splitDeclarationSegments(items []declarationToken) [][]declarationToken {
	var segments [][]declarationToken
	start := 0
	depth := 0
	for index, item := range items {
		//nolint:exhaustive // Segment splitting only tracks delimiters and separators.
		switch item.kind {
		case token.LParen, token.LBracket, token.LBrace:
			depth++
		case token.RParen, token.RBracket, token.RBrace:
			if depth > 0 {
				depth--
			}
		case token.Comma:
			if depth == 0 {
				segments = append(segments, items[start:index])
				start = index + 1
			}
		case token.Semicolon:
			if depth == 0 {
				segments = append(segments, items[start:index])
				return segments
			}
		}
	}
	if start < len(items) {
		segments = append(segments, items[start:])
	}
	return segments
}

func declarationTokenContains(items []declarationToken, kind token.Kind) bool {
	return declarationTokenIndex(items, kind) >= 0
}

func declarationTokenIndex(items []declarationToken, kind token.Kind) int {
	for index, item := range items {
		if item.kind == kind {
			return index
		}
	}
	return -1
}

func topLevelDeclarationToken(items []declarationToken, kind token.Kind) int {
	depth := 0
	for index, item := range items {
		if item.kind == kind && depth == 0 {
			return index
		}
		//nolint:exhaustive // Top-level lookup only tracks nested delimiters.
		switch item.kind {
		case token.LParen, token.LBracket, token.LBrace:
			depth++
		case token.RParen, token.RBracket, token.RBrace:
			if depth > 0 {
				depth--
			}
		}
	}
	return -1
}

func matchingDeclarationBracket(items []declarationToken, open int) int {
	depth := 0
	for index := open; index < len(items); index++ {
		//nolint:exhaustive // Bracket matching only tracks bracket tokens.
		switch items[index].kind {
		case token.LBracket:
			depth++
		case token.RBracket:
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}
