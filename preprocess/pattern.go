package preprocess

import (
	"bytes"
	"strings"

	"github.com/pawnkit/pawn-parser/lexer"
	"github.com/pawnkit/pawn-parser/token"
)

type pawnPatternMatch struct {
	end  int
	args map[int]ByteRange
}

// matchPawnPattern mirrors PawnCC 3.10.10's substpattern(): the alphanumeric
// prefix is exact, %0-%9 capture until the following pattern byte, strings and
// balanced groups are opaque while searching, and whitespace before most
// punctuation is insignificant.
func matchPawnPattern(source, pattern string, semicolons bool, control byte) (pawnPatternMatch, bool) {
	prefix := 0
	for prefix < len(pattern) && pawnAlphanum(pattern[prefix]) {
		prefix++
	}
	if prefix == 0 || len(source) < prefix || source[:prefix] != pattern[:prefix] {
		return pawnPatternMatch{}, false
	}

	sourceIndex := prefix
	patternIndex := prefix
	args := make(map[int]ByteRange)
	for patternIndex < len(pattern) {
		current := pattern[patternIndex]
		switch {
		case current == '%':
			if patternIndex+2 >= len(pattern) || pattern[patternIndex+1] < '0' || pattern[patternIndex+1] > '9' {
				return pawnPatternMatch{}, false
			}
			label := int(pattern[patternIndex+1] - '0')
			delimiter := pattern[patternIndex+2]
			patternIndex += 3
			start := sourceIndex
			for sourceIndex < len(source) && source[sourceIndex] != delimiter {
				if next, ok := skipPawnQuoted(source, sourceIndex, control); ok {
					sourceIndex = next
					continue
				}
				if next, ok := skipPawnGroup(source, sourceIndex, control); ok {
					sourceIndex = next
					continue
				}
				sourceIndex++
			}
			args[label] = ByteRange{Start: start, End: sourceIndex}
			if sourceIndex < len(source) && source[sourceIndex] == delimiter {
				sourceIndex++
				continue
			}
			if delimiter == ';' && patternIndex == len(pattern) && !semicolons {
				continue
			}
			return pawnPatternMatch{}, false

		case current == ';' && patternIndex+1 == len(pattern) && !semicolons:
			for sourceIndex < len(source) && source[sourceIndex] <= ' ' {
				sourceIndex++
			}
			if sourceIndex < len(source) {
				if source[sourceIndex] != ';' {
					return pawnPatternMatch{}, false
				}
				sourceIndex++
			}
			patternIndex++

		default:
			if !pawnAlphanum(current) && patternIndex > 0 && pattern[patternIndex-1] != current {
				for sourceIndex < len(source) && source[sourceIndex] <= ' ' {
					sourceIndex++
				}
			}
			if sourceIndex >= len(source) || source[sourceIndex] != current {
				return pawnPatternMatch{}, false
			}
			sourceIndex++
			patternIndex++
		}
	}

	if patternIndex != len(pattern) {
		return pawnPatternMatch{}, false
	}
	if patternIndex > 0 && pawnAlphanum(pattern[patternIndex-1]) && sourceIndex < len(source) && pawnAlphanum(source[sourceIndex]) {
		return pawnPatternMatch{}, false
	}
	return pawnPatternMatch{end: sourceIndex, args: args}, true
}

func skipPawnQuoted(source string, start int, control byte) (int, bool) {
	if start >= len(source) {
		return start, false
	}
	quoteIndex := start
	if source[start] == '!' && start+1 < len(source) && source[start+1] == '"' {
		quoteIndex++
	}
	quote := source[quoteIndex]
	if quote != '"' && quote != '\'' {
		return start, false
	}
	for index := quoteIndex + 1; index < len(source); index++ {
		if source[index] == control && index+1 < len(source) {
			index++
			continue
		}
		if source[index] == quote {
			return index + 1, true
		}
	}
	return len(source), true
}

func skipPawnGroup(source string, start int, control byte) (int, bool) {
	if start >= len(source) || !strings.ContainsRune("([{", rune(source[start])) {
		return start, false
	}
	stack := []byte{source[start]}
	for index := start + 1; index < len(source); {
		if next, ok := skipPawnQuoted(source, index, control); ok {
			index = next
			continue
		}
		switch source[index] {
		case '(', '[', '{':
			stack = append(stack, source[index])
		case ')', ']', '}':
			open := stack[len(stack)-1]
			if open == '(' && source[index] == ')' || open == '[' && source[index] == ']' || open == '{' && source[index] == '}' {
				stack = stack[:len(stack)-1]
				if len(stack) == 0 {
					return index + 1, true
				}
			}
		}
		index++
	}
	return len(source), true
}

type replacementSegment struct {
	start  int
	end    int
	parent *token.Origin
}

type replacementText struct {
	text     string
	segments []replacementSegment
}

func (e *engine) substituteMacroBody(macro Macro, input string, match pawnPatternMatch, captureOrigin func(ByteRange) *token.Origin) replacementText {
	var output strings.Builder
	output.Grow(len(macro.BodyText))
	var segments []replacementSegment
	appendText := func(text string, parent *token.Origin) {
		if text == "" {
			return
		}
		start := output.Len()
		output.WriteString(text)
		segments = append(segments, replacementSegment{start: start, end: output.Len(), parent: parent})
	}
	literalOrigin := func(start, end int) *token.Origin {
		if int(macro.File) >= len(e.files) || macro.BodySpan.Start+start > macro.BodySpan.Start+end {
			return nil
		}
		content := e.files[macro.File].Content
		absoluteStart := macro.BodySpan.Start + start
		absoluteEnd := macro.BodySpan.Start + end
		if absoluteStart < 0 || absoluteEnd > len(content) {
			return nil
		}
		lineMap := token.NewLineMap(content)
		return &token.Origin{Span: token.Span{
			File:  macro.File,
			Start: lineMap.Position(uint32(absoluteStart)), //nolint:gosec // indexes macro definition content.
			End:   lineMap.Position(uint32(absoluteEnd)),   //nolint:gosec // indexes macro definition content.
		}}
	}

	inString := false
	literalStart := 0
	for index := 0; index < len(macro.BodyText); index++ {
		if macro.BodyText[index] == '"' {
			inString = !inString
			continue
		}
		if !inString && macro.BodyText[index] == '%' && index+1 < len(macro.BodyText) && macro.BodyText[index+1] >= '0' && macro.BodyText[index+1] <= '9' {
			appendText(macro.BodyText[literalStart:index], literalOrigin(literalStart, index))
			capture, ok := match.args[int(macro.BodyText[index+1]-'0')]
			if ok && capture.Start >= 0 && capture.Start <= capture.End && capture.End <= len(input) {
				appendText(input[capture.Start:capture.End], captureOrigin(capture))
			} else {
				appendText(macro.BodyText[index:index+2], literalOrigin(index, index+2))
			}
			index++
			literalStart = index + 1
			continue
		}
	}
	appendText(macro.BodyText[literalStart:], literalOrigin(literalStart, len(macro.BodyText)))
	return replacementText{text: output.String(), segments: segments}
}

func replacementTokens(replacement replacementText, inv token.Span, macroName string) []ptok {
	source := []byte(replacement.text)
	tokens := lexer.Tokenize(source)
	if len(tokens) != 0 && tokens[len(tokens)-1].Kind == token.EOF {
		tokens = tokens[:len(tokens)-1]
	}
	items := toPtoks(source, tokens, inv.File)
	for index := range items {
		var parent *token.Origin
		for _, segment := range replacement.segments {
			if segment.end > items[index].Start.Offset && segment.start < items[index].End.Offset {
				parent = segment.parent
				break
			}
		}
		items[index].Origin = &token.Origin{Span: inv, Macro: macroName, Parent: parent}
	}
	return items
}

type patternTokenInput struct {
	text             string
	tokenStart       []int
	tokenEnd         []int
	tokenTrailingEnd []int
}

func makePatternTokenInput(tokens []ptok, start int) patternTokenInput {
	var output strings.Builder
	starts := make([]int, 0, len(tokens)-start)
	ends := make([]int, 0, len(tokens)-start)
	trailingEnds := make([]int, 0, len(tokens)-start)
	for index := start; index < len(tokens); index++ {
		starts = append(starts, output.Len())
		output.WriteString(tokens[index].text)
		ends = append(ends, output.Len())
		trailing := tokens[index].trailing
		if newline := strings.IndexAny(trailing, "\r\n"); newline >= 0 {
			output.WriteString(trailing[:newline])
			trailingEnds = append(trailingEnds, output.Len())
			break
		}
		output.WriteString(trailing)
		trailingEnds = append(trailingEnds, output.Len())
	}
	return patternTokenInput{text: output.String(), tokenStart: starts, tokenEnd: ends, tokenTrailingEnd: trailingEnds}
}

func (input patternTokenInput) consumedTokens(end int) (consumed, suffixOffset, trailingConsumed int, aligned bool) {
	for index, tokenEnd := range input.tokenEnd {
		if tokenEnd == end {
			return index + 1, -1, 0, true
		}
		if tokenEnd > end {
			if end > input.tokenStart[index] {
				return index + 1, end - input.tokenStart[index], 0, true
			}
			return consumed, -1, 0, consumed > 0 && end <= input.tokenStart[index]
		}
		if index < len(input.tokenTrailingEnd) && end <= input.tokenTrailingEnd[index] {
			return index + 1, -1, end - tokenEnd, true
		}
		consumed = index + 1
	}
	return consumed, -1, 0, consumed > 0 && end <= len(input.text)
}

func (input patternTokenInput) captureOrigin(tokens []ptok, start int, capture ByteRange) *token.Origin {
	for index := range input.tokenStart {
		if input.tokenEnd[index] <= capture.Start || input.tokenStart[index] >= capture.End {
			continue
		}
		item := tokens[start+index]
		if item.Origin != nil {
			return item.Origin
		}
		return &token.Origin{Span: token.Span{File: item.file, Start: item.Start, End: item.End}}
	}
	return nil
}

func (e *engine) expandPatternAt(f *frame, name token.Token, macro Macro) bool {
	lineEnd := sourceLogicalLineEnd(f.source, name.Start.Offset)
	rawInput := f.source[name.Start.Offset:lineEnd]
	maskedInput := stripPatternComments(rawInput, e.opts.ControlChar)
	input := string(maskedInput)
	match, ok := matchPawnPattern(input, macro.Pattern, e.opts.Semicolons, e.opts.ControlChar)
	if !ok {
		return false
	}
	if match.end > 0 && match.end <= len(maskedInput) && maskedInput[match.end-1] == '\n' {
		e.joinListingWithNextLine()
	}

	absoluteEnd := name.Start.Offset + match.end
	if len(maskedInput) < len(rawInput) && match.end == len(maskedInput) {
		absoluteEnd = name.Start.Offset + len(rawInput)
	}
	consumed := name
	for !f.atEnd() && f.cur().Start.Offset < absoluteEnd {
		consumed = f.advance()
	}
	lineMap := token.NewLineMap(f.source)
	invocation := token.Span{
		File: f.fileIndex, Start: name.Start,
		End: lineMap.Position(uint32(absoluteEnd)), //nolint:gosec // absoluteEnd indexes f.source.
	}
	e.recordInvocation(invocation)
	captureOrigin := func(capture ByteRange) *token.Origin {
		return &token.Origin{Span: token.Span{
			File:  f.fileIndex,
			Start: lineMap.Position(uint32(name.Start.Offset + capture.Start)), //nolint:gosec // indexes f.source.
			End:   lineMap.Position(uint32(name.Start.Offset + capture.End)),   //nolint:gosec // indexes f.source.
		}}
	}
	replacement := e.substituteMacroBody(macro, input, match, captureOrigin)
	items := replacementTokens(replacement, invocation, macro.Name)
	if absoluteEnd < consumed.End.Offset {
		original := toPtok(f.source, consumed, f.fileIndex)
		items = append(items, splitPtokSuffix(original, absoluteEnd-consumed.Start.Offset))
	}
	boundaryTrailing := toPtok(f.source, consumed, f.fileIndex).trailing
	if absoluteEnd > consumed.End.Offset {
		consumedTrivia := absoluteEnd - consumed.End.Offset
		boundaryTrailing = boundaryTrailing[min(consumedTrivia, len(boundaryTrailing)):]
	}
	e.expandWithFrameRemainder(f, items, consumed, boundaryTrailing, hideSet{}.with(macro.Name), 1)
	return true
}

func stripPatternComments(source []byte, control byte) []byte {
	output := bytes.Clone(source)
	var quote byte
	for index := 0; index < len(output); {
		if quote != 0 {
			if output[index] == control && index+1 < len(output) {
				index += 2
				continue
			}
			if output[index] == quote {
				quote = 0
			}
			index++
			continue
		}
		switch {
		case output[index] == '"' || output[index] == '\'':
			quote = output[index]
			index++
		case index+1 < len(output) && output[index] == '/' && output[index+1] == '/':
			// PawnCC replaces the first slash with a newline and terminates
			// the logical buffer at the second, so comment text does not add
			// padding to a captured end-of-line argument.
			output[index] = '\n'
			return output[:index+1]
		case index+1 < len(output) && output[index] == '/' && output[index+1] == '*':
			output[index], output[index+1] = ' ', ' '
			index += 2
			for index < len(output) {
				if index+1 < len(output) && output[index] == '*' && output[index+1] == '/' {
					output[index], output[index+1] = ' ', ' '
					index += 2
					break
				}
				if output[index] != '\n' && output[index] != '\r' {
					output[index] = ' '
				}
				index++
			}
		default:
			index++
		}
	}
	return output
}

func sourceLogicalLineEnd(source []byte, offset int) int {
	offset = min(max(offset, 0), len(source))
	for offset < len(source) {
		relative := bytes.IndexByte(source[offset:], '\n')
		if relative < 0 {
			return len(source)
		}
		lineEnd := offset + relative
		trimmed := lineEnd
		if trimmed > offset && source[trimmed-1] == '\r' {
			trimmed--
		}
		for trimmed > offset && (source[trimmed-1] == ' ' || source[trimmed-1] == '\t') {
			trimmed--
		}
		if trimmed > offset && source[trimmed-1] == '\\' {
			offset = lineEnd + 1
			continue
		}
		return min(lineEnd+1, len(source))
	}
	return len(source)
}

func (e *engine) expandPatternSlice(tokens []ptok, start int, macro Macro) ([]ptok, int, string, bool) {
	input := makePatternTokenInput(tokens, start)
	match, ok := matchPawnPattern(input.text, macro.Pattern, e.opts.Semicolons, e.opts.ControlChar)
	if !ok {
		return nil, start, "", false
	}
	consumed, suffixOffset, trailingConsumed, aligned := input.consumedTokens(match.end)
	if !aligned || consumed == 0 || start+consumed > len(tokens) {
		return nil, start, "", false
	}
	invocation := nestedInvocationSpan(tokens[start], tokens[start+consumed-1])
	replacement := e.substituteMacroBody(macro, input.text, match, func(capture ByteRange) *token.Origin {
		return input.captureOrigin(tokens, start, capture)
	})
	items := replacementTokens(replacement, invocation, macro.Name)
	if suffixOffset >= 0 {
		items = append(items, splitPtokSuffix(tokens[start+consumed-1], suffixOffset))
	}
	boundaryTrailing := tokens[start+consumed-1].trailing
	if trailingConsumed > 0 {
		boundaryTrailing = boundaryTrailing[min(trailingConsumed, len(boundaryTrailing)):]
	}
	return items, start + consumed, boundaryTrailing, true
}

func splitPtokSuffix(item ptok, offset int) ptok {
	offset = min(max(offset, 0), len(item.text))
	item.text = item.text[offset:]
	item.Start.Offset += offset
	item.Start.Col += offset
	item.LeadingTrivia = nil
	item.TrailingTrivia = nil
	item.leading = ""
	item.trailing = ""
	return item
}
