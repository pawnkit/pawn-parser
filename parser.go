// Package parser is a reusable concrete-syntax-tree parser for the Pawn
// language used by SA-MP and open.mp projects.
package parser

import (
	"sort"
	"strconv"

	"github.com/pawnkit/pawn-parser/lexer"
	"github.com/pawnkit/pawn-parser/token"
)

// File is the result of parsing one Pawn source file/buffer.
type File struct {
	Source []byte
	Tokens []token.Token
	Root   *Node

	Broken bool

	Diagnostics []Diagnostic
}

// HasParseErrors reports whether parsing produced a syntax diagnostic or an
// unrecoverable/broken result.
func (f *File) HasParseErrors() bool {
	return f == nil || f.Root == nil || f.Broken || f.Root.HasError || len(f.Diagnostics) > 0
}

// Parse parses source into a File. Parse errors are reported via
// File.Broken and Node.HasError.
func Parse(source []byte) *File {
	return ParseWithOptions(source, ParseOptions{})
}

// ParseOptions controls retained parse data.
type ParseOptions struct {
	DiscardTokens bool
	DiscardTrivia bool
}

// ParseWithOptions parses source with retention options.
func ParseWithOptions(source []byte, options ParseOptions) *File {
	toks := lexer.Tokenize(source)
	return parseTokens(source, toks, options)
}

// ParseTokens parses a caller-provided token stream. It is useful for parsing
// preprocessed tokens whose Origin fields retain expansion history.
func ParseTokens(source []byte, toks []token.Token) *File {
	return ParseTokensWithOptions(source, toks, ParseOptions{})
}

// ParseTokensWithOptions parses tokens with retention options.
func ParseTokensWithOptions(source []byte, toks []token.Token, options ParseOptions) *File {
	return parseTokens(source, toks, options)
}

func parseTokens(source []byte, toks []token.Token, options ParseOptions) *File {
	if len(toks) == 0 || toks[len(toks)-1].Kind != token.EOF {
		end := token.Position{Offset: len(source)}
		toks = append(append([]token.Token(nil), toks...), token.Token{
			Kind:  token.EOF,
			Start: end,
			End:   end,
		})
	}
	p := newPointerParser(source, toks, nil)
	root := p.parseSourceFile()
	p.buildDiagnosticCoverage()
	p.ensureErrorDiagnostics(root)
	sort.SliceStable(p.diagnostics, func(i, j int) bool {
		if p.diagnostics[i].Range.Start != p.diagnostics[j].Range.Start {
			return p.diagnostics[i].Range.Start < p.diagnostics[j].Range.Start
		}
		return p.diagnostics[i].Range.End < p.diagnostics[j].Range.End
	})
	if options.DiscardTrivia {
		p.sink.storage.arena.discardTrivia()
	}
	toks = retainedTokens(toks, options)
	return &File{Source: source, Tokens: toks, Root: root, Broken: p.broken, Diagnostics: p.diagnostics}
}

func retainedTokens(toks []token.Token, options ParseOptions) []token.Token {
	if options.DiscardTokens {
		return nil
	}
	if options.DiscardTrivia {
		withoutTrivia := make([]token.Token, len(toks))
		copy(withoutTrivia, toks)
		for i := range withoutTrivia {
			withoutTrivia[i].LeadingTrivia = nil
			withoutTrivia[i].TrailingTrivia = nil
		}
		return withoutTrivia
	}
	return toks
}

type parser[N comparable, S nodeSink[N]] struct {
	source    []byte
	toks      []token.Token
	pos       int
	broken    bool
	condDepth int
	depth     int

	branchTop bool

	allowMissingTrailingSemi bool

	suppressTagCast bool
	knownTags       map[string]struct{}

	sink S

	diagnostics []Diagnostic
	depthError  bool

	diagnosticRanges []diagnosticRange
	diagnosticPoints []int
}

type parserStorage struct {
	arena    nodeArena
	fields   fieldArena
	entries  fieldEntryArena
	children childArena
	trivia   parserTriviaArena
}

type parserStorageMark struct {
	arena    nodeArenaMark
	fields   nodeArenaMark
	entries  nodeArenaMark
	children nodeArenaMark
	trivia   nodeArenaMark
}

func newPointerParser(source []byte, toks []token.Token, storage *parserStorage) *parser[*Node, pointerNodeSink] {
	if storage == nil {
		storage = new(parserStorage)
	}
	p := &parser[*Node, pointerNodeSink]{source: source, toks: toks}
	p.sink.storage = storage
	p.sink.source = source
	return p
}

func (s *parserStorage) mark() parserStorageMark {
	return parserStorageMark{
		arena: s.arena.mark(), fields: s.fields.mark(),
		entries: s.entries.mark(), children: s.children.mark(), trivia: s.trivia.mark(),
	}
}

func (s *parserStorage) rewind(mark parserStorageMark) {
	s.arena.rewind(mark.arena)
	s.fields.rewind(mark.fields)
	s.entries.rewind(mark.entries)
	s.children.rewind(mark.children)
	s.trivia.rewind(mark.trivia)
}

const (
	maxParseDepth  = 1000
	maxDiagnostics = 1024
)

func (p *parser[N, S]) enterDepth() bool {
	p.depth++
	if p.depth > maxParseDepth && !p.depthError {
		p.depthError = true
		found := p.cur()
		p.emitDiagnostic(Diagnostic{
			Code: DiagnosticMaximumDepth, Message: "maximum parse depth exceeded",
			Range: tokenRange(found), Found: found,
			Recovery: suggestedRecovery(tokenRange(found)),
		})
	}
	return p.depth <= maxParseDepth
}

func (p *parser[N, S]) exitDepth() {
	p.depth--
}

const (
	initialNodeBlockSize = 64
	maxNodeBlockSize     = 1024
)

// nodeArena provides pointer-stable storage.
type nodeArena struct {
	blocks [][]Node
	next   int
}

type nodeArenaMark struct {
	blocks int
	next   int
}

type fieldArena struct {
	blocks [][]nodeFieldData
	next   int
}

func (a *fieldArena) alloc() *nodeFieldData {
	if len(a.blocks) == 0 || a.next == len(a.blocks[len(a.blocks)-1]) {
		size := 32
		if len(a.blocks) != 0 {
			size = min(len(a.blocks[len(a.blocks)-1])*2, 1024)
		}
		a.blocks = append(a.blocks, make([]nodeFieldData, size))
		a.next = 0
	}
	value := &a.blocks[len(a.blocks)-1][a.next]
	a.next++
	return value
}

func (a *fieldArena) mark() nodeArenaMark {
	return nodeArenaMark{blocks: len(a.blocks), next: a.next}
}

func (a *fieldArena) rewind(mark nodeArenaMark) {
	if mark.blocks == 0 {
		a.blocks = nil
		a.next = 0
		return
	}
	clear(a.blocks[mark.blocks-1][mark.next:])
	a.blocks = a.blocks[:mark.blocks]
	a.next = mark.next
}

type childArena struct {
	blocks [][]*Node
	next   int
}

type fieldEntryArena struct {
	blocks [][]fieldEntry
	next   int
}

//nolint:dupl // Typed arenas intentionally share growth rules.
func (a *fieldEntryArena) alloc(size int) []fieldEntry {
	if len(a.blocks) == 0 || len(a.blocks[len(a.blocks)-1])-a.next < size {
		blockSize := 32
		if len(a.blocks) != 0 {
			blockSize = min(len(a.blocks[len(a.blocks)-1])*2, 1024)
		}
		a.blocks = append(a.blocks, make([]fieldEntry, max(size, blockSize)))
		a.next = 0
	}
	entries := a.blocks[len(a.blocks)-1][a.next : a.next+size : a.next+size]
	a.next += size
	return entries
}

func (a *fieldEntryArena) mark() nodeArenaMark {
	return nodeArenaMark{blocks: len(a.blocks), next: a.next}
}

func (a *fieldEntryArena) rewind(mark nodeArenaMark) {
	if mark.blocks == 0 {
		a.blocks = nil
		a.next = 0
		return
	}
	clear(a.blocks[mark.blocks-1][mark.next:])
	a.blocks = a.blocks[:mark.blocks]
	a.next = mark.next
}

type parserTriviaArena struct {
	blocks [][]token.Trivia
	next   int
}

func (a *parserTriviaArena) alloc(size int) []token.Trivia {
	if size == 0 {
		return nil
	}
	if len(a.blocks) == 0 || len(a.blocks[len(a.blocks)-1])-a.next < size {
		blockSize := 64
		if len(a.blocks) != 0 {
			blockSize = min(len(a.blocks[len(a.blocks)-1])*2, 4096)
		}
		a.blocks = append(a.blocks, make([]token.Trivia, max(size, blockSize)))
		a.next = 0
	}
	trivia := a.blocks[len(a.blocks)-1][a.next : a.next+size : a.next+size]
	a.next += size
	return trivia
}

func (a *parserTriviaArena) mark() nodeArenaMark {
	return nodeArenaMark{blocks: len(a.blocks), next: a.next}
}

func (a *parserTriviaArena) rewind(mark nodeArenaMark) {
	if mark.blocks == 0 {
		a.blocks = nil
		a.next = 0
		return
	}
	clear(a.blocks[mark.blocks-1][mark.next:])
	a.blocks = a.blocks[:mark.blocks]
	a.next = mark.next
}

func (a *childArena) alloc(size int) []*Node {
	if size == 0 {
		return nil
	}
	if len(a.blocks) == 0 || len(a.blocks[len(a.blocks)-1])-a.next < size {
		blockSize := 64
		if len(a.blocks) != 0 {
			blockSize = min(len(a.blocks[len(a.blocks)-1])*2, 4096)
		}
		a.blocks = append(a.blocks, make([]*Node, max(size, blockSize)))
		a.next = 0
	}
	children := a.blocks[len(a.blocks)-1][a.next : a.next+size : a.next+size]
	a.next += size
	return children
}

func (a *childArena) mark() nodeArenaMark {
	return nodeArenaMark{blocks: len(a.blocks), next: a.next}
}

func (a *childArena) rewind(mark nodeArenaMark) {
	if mark.blocks == 0 {
		a.blocks = nil
		a.next = 0
		return
	}
	clear(a.blocks[mark.blocks-1][mark.next:])
	a.blocks = a.blocks[:mark.blocks]
	a.next = mark.next
}

func (a *nodeArena) alloc() *Node {
	if len(a.blocks) == 0 || a.next == len(a.blocks[len(a.blocks)-1]) {
		size := initialNodeBlockSize
		if len(a.blocks) > 0 {
			size = min(len(a.blocks[len(a.blocks)-1])*2, maxNodeBlockSize)
		}
		a.blocks = append(a.blocks, make([]Node, size))
		a.next = 0
	}
	block := a.blocks[len(a.blocks)-1]
	n := &block[a.next]
	a.next++
	return n
}

func (a *nodeArena) mark() nodeArenaMark {
	return nodeArenaMark{blocks: len(a.blocks), next: a.next}
}

func (a *nodeArena) rewind(mark nodeArenaMark) {
	if mark.blocks == 0 {
		for i := range a.blocks {
			clear(a.blocks[i])
		}
		a.blocks = nil
		a.next = 0
		return
	}
	clear(a.blocks[mark.blocks-1][mark.next:])
	for i := mark.blocks; i < len(a.blocks); i++ {
		clear(a.blocks[i])
	}
	a.blocks = a.blocks[:mark.blocks]
	a.next = mark.next
}

func (a *nodeArena) discardTrivia() {
	for blockIndex, block := range a.blocks {
		if blockIndex == len(a.blocks)-1 {
			block = block[:a.next]
		}
		for i := range block {
			block[i].Leading = nil
			block[i].Trailing = nil
			block[i].Tok.LeadingTrivia = nil
			block[i].Tok.TrailingTrivia = nil
		}
	}
}

func (p *parser[N, S]) missingSemiOK() bool {
	return p.at(token.RBrace) || p.allowMissingTrailingSemi && p.atEnd()
}

func (p *parser[N, S]) abortIfSharedAcrossBranch() {
	if p.condDepth == 0 {
		return
	}
	if !p.at(token.Hash) {
		return
	}
	switch p.peekDirectiveKeyword() {
	case dirElseif, dirElse, dirEndif:
		panic(condAbort{})
	}
}

func (p *parser[N, S]) cur() token.Token {
	return p.toks[p.pos]
}

func (p *parser[N, S]) peek(offset int) token.Token {
	idx := max(p.pos+offset, 0)
	if idx >= len(p.toks) {
		return p.toks[len(p.toks)-1]
	}
	return p.toks[idx]
}

func (p *parser[N, S]) curKind() token.Kind {
	return p.toks[p.pos].Kind
}

func (p *parser[N, S]) peekKind(offset int) token.Kind {
	idx := max(p.pos+offset, 0)
	if idx >= len(p.toks) {
		return p.toks[len(p.toks)-1].Kind
	}
	return p.toks[idx].Kind
}

func (p *parser[N, S]) at(k token.Kind) bool {
	return p.curKind() == k
}

func (p *parser[N, S]) atEnd() bool {
	return p.curKind() == token.EOF
}

func (p *parser[N, S]) advance() token.Token {
	t := p.cur()
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	return t
}

func tokenRange(tok token.Token) ByteRange {
	return ByteRange{Start: tok.Start.Offset, End: tok.End.Offset}
}

func suggestedRecovery(r ByteRange) Recovery {
	return Recovery{Kind: RecoveryNone, Range: r, Confidence: RecoverySuggested}
}

func (p *parser[N, S]) emitDiagnostic(d Diagnostic) {
	if len(p.diagnostics) >= maxDiagnostics {
		return
	}
	d.Expected = append([]token.Kind(nil), d.Expected...)
	p.diagnostics = append(p.diagnostics, d)
}

func (p *parser[N, S]) ensureErrorDiagnostics(root N) {
	var visit func(N) bool
	visit = func(node N) bool {
		if node == p.sink.Nil() || !p.sink.HasError(node) {
			return false
		}
		childHasError := false
		for _, child := range p.sink.Children(node) {
			if visit(child) {
				childHasError = true
			}
		}
		if !childHasError && !p.diagnosticCovers(node) {
			found := p.tokenAtOffset(p.sink.Start(node))
			message := p.sink.ErrorMessage(node)
			if message == "" {
				message = "syntax error in " + p.sink.Kind(node).String()
			}
			r := tokenRange(found)
			p.emitDiagnostic(Diagnostic{
				Code: DiagnosticSyntaxError, Message: message,
				Range: r, Found: found, Expected: p.sink.ErrorExpected(node), Recovery: suggestedRecovery(r),
			})
		}
		return true
	}
	visit(root)
}

func (p *parser[N, S]) diagnosticCovers(node N) bool {
	start, end := p.sink.Start(node), p.sink.End(node)
	point := sort.SearchInts(p.diagnosticPoints, start)
	if point < len(p.diagnosticPoints) && p.diagnosticPoints[point] <= end {
		return true
	}
	rangeEnd := sort.Search(len(p.diagnosticRanges), func(i int) bool {
		return p.diagnosticRanges[i].start >= end
	})
	return rangeEnd > 0 && p.diagnosticRanges[rangeEnd-1].maxEnd > start
}

type diagnosticRange struct {
	start  int
	end    int
	maxEnd int
}

func (p *parser[N, S]) buildDiagnosticCoverage() {
	for _, diagnostic := range p.diagnostics {
		if diagnostic.Range.Start == diagnostic.Range.End {
			p.diagnosticPoints = append(p.diagnosticPoints, diagnostic.Range.Start)
			continue
		}
		p.diagnosticRanges = append(p.diagnosticRanges, diagnosticRange{
			start: diagnostic.Range.Start,
			end:   diagnostic.Range.End,
		})
	}
	sort.Ints(p.diagnosticPoints)
	sort.Slice(p.diagnosticRanges, func(i, j int) bool {
		return p.diagnosticRanges[i].start < p.diagnosticRanges[j].start
	})
	maxEnd := 0
	for i := range p.diagnosticRanges {
		maxEnd = max(maxEnd, p.diagnosticRanges[i].end)
		p.diagnosticRanges[i].maxEnd = maxEnd
	}
}

func (p *parser[N, S]) tokenAtOffset(offset int) token.Token {
	idx := sort.Search(len(p.toks), func(i int) bool {
		return p.toks[i].End.Offset >= offset
	})
	if idx < len(p.toks) {
		return p.toks[idx]
	}
	return p.toks[len(p.toks)-1]
}

func (p *parser[N, S]) emitMissingToken(expected token.Kind, context string) {
	found := p.cur()
	message := "expected " + strconv.Quote(expected.String())
	if context != "" {
		message += " to close " + context
	}
	p.emitDiagnostic(Diagnostic{
		Code: DiagnosticMissingToken, Message: message,
		Range: ByteRange{Start: found.Start.Offset, End: found.Start.Offset},
		Found: found, Expected: []token.Kind{expected},
		Recovery: Recovery{
			Kind:        RecoveryInsert,
			Range:       ByteRange{Start: found.Start.Offset, End: found.Start.Offset},
			Replacement: expected.String(), Confidence: RecoveryExact,
		},
	})
}

func (p *parser[N, S]) emitMissing(code DiagnosticCode, message string, expected ...token.Kind) {
	found := p.cur()
	r := ByteRange{Start: found.Start.Offset, End: found.Start.Offset}
	p.emitDiagnostic(Diagnostic{
		Code: code, Message: message, Range: r, Found: found,
		Expected: expected, Recovery: suggestedRecovery(r),
	})
}

func (p *parser[N, S]) makeRecoveryNode(start, end int, found token.Token, grammar itemGrammar[N, S], exactRemove bool) N {
	n := p.recoveryNode(start, end, found, grammar.recoveryContext, grammar.recoveryExpected)
	foundRange := tokenRange(found)
	code := DiagnosticUnexpectedToken
	recovery := suggestedRecovery(foundRange)
	if exactRemove {
		code = DiagnosticUnexpectedDelimiter
		recovery = Recovery{Kind: RecoveryRemove, Range: foundRange, Confidence: RecoveryExact}
	}
	message := p.sink.ErrorMessage(n)
	if message == "" {
		message = "unexpected " + strconv.Quote(found.Kind.String())
	}
	p.emitDiagnostic(Diagnostic{
		Code: code, Message: message, Range: foundRange,
		Found: found, Expected: grammar.recoveryExpected, Recovery: recovery,
	})
	return n
}

func (p *parser[N, S]) recoverStuckItem(grammar itemGrammar[N, S]) N {
	start := p.cur().Start.Offset
	startIdx := p.pos
	found := p.cur()
	depth := 0
	var live []bool
	liveFalseCount := 0
	allLive := func() bool {
		return liveFalseCount == 0
	}
	for !p.atEnd() {
		if p.cur().Kind == token.Hash {
			live, liveFalseCount = p.updateRecoveryLiveness(live, liveFalseCount)
		}
		counting := allLive()
		switch p.cur().Kind {
		case token.LBrace, token.LParen, token.LBracket:
			if counting {
				depth++
			}
			p.advance()
		case token.Semicolon:
			if depth > 0 {
				p.advance()
				continue
			}
			if grammar.preserveRecoverySemicolon {
				return p.stuckBoundary(start, startIdx, found, grammar)
			}
			last := p.advance()
			return p.makeRecoveryNode(start, last.End.Offset, found, grammar, false)
		case token.RBrace, token.RParen, token.RBracket:
			if depth > 0 {
				if counting {
					depth--
				}
				p.advance()
				continue
			}
			return p.stuckBoundary(start, startIdx, found, grammar)
		case token.Hash:
			if depth > 0 {
				p.advance()
				continue
			}
			return p.stuckBoundary(start, startIdx, found, grammar)
		default:
			p.advance()
		}
	}
	if p.pos == startIdx {
		p.broken = true
		p.emitDiagnostic(Diagnostic{
			Code:    DiagnosticUnrecoverable,
			Message: "parser could not recover before end of file",
			Range:   tokenRange(found), Found: found, Recovery: suggestedRecovery(tokenRange(found)),
		})
		return p.sink.Nil()
	}
	return p.makeRecoveryNode(start, p.toks[len(p.toks)-1].End.Offset, found, grammar, false)
}

func (p *parser[N, S]) updateRecoveryLiveness(live []bool, liveFalseCount int) ([]bool, int) {
	switch p.peekDirectiveKeyword() {
	case dirIf:
		live = append(live, true)
	case dirElseif, dirElse:
		if len(live) > 0 {
			if live[len(live)-1] {
				liveFalseCount++
			}
			live[len(live)-1] = false
		}
	case dirEndif:
		if len(live) > 0 {
			if !live[len(live)-1] {
				liveFalseCount--
			}
			live = live[:len(live)-1]
		}
	}
	return live, liveFalseCount
}

func (p *parser[N, S]) stuckBoundary(start, startIdx int, found token.Token, grammar itemGrammar[N, S]) N {
	if p.pos == startIdx {
		p.broken = true
		last := p.advance()
		exactRemove := found.Kind == token.RBrace || found.Kind == token.RParen || found.Kind == token.RBracket
		return p.makeRecoveryNode(start, last.End.Offset, found, grammar, exactRemove)
	}
	return p.makeRecoveryNode(start, p.toks[p.pos-1].End.Offset, found, grammar, false)
}
