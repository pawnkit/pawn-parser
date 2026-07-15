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
	toks := lexer.Tokenize(source)
	return ParseTokens(source, toks)
}

// ParseTokens parses a caller-provided token stream. It is useful for parsing
// preprocessed tokens whose Origin fields retain expansion history.
func ParseTokens(source []byte, toks []token.Token) *File {
	if len(toks) == 0 || toks[len(toks)-1].Kind != token.EOF {
		end := token.Position{Offset: len(source)}
		toks = append(append([]token.Token(nil), toks...), token.Token{
			Kind:  token.EOF,
			Start: end,
			End:   end,
		})
	}
	p := &parser{source: source, toks: toks, arena: make([]Node, 0, len(source)/4+16)}
	root := p.parseSourceFile()
	sort.SliceStable(p.diagnostics, func(i, j int) bool {
		if p.diagnostics[i].Range.Start != p.diagnostics[j].Range.Start {
			return p.diagnostics[i].Range.Start < p.diagnostics[j].Range.Start
		}
		return p.diagnostics[i].Range.End < p.diagnostics[j].Range.End
	})
	return &File{Source: source, Tokens: toks, Root: root, Broken: p.broken, Diagnostics: p.diagnostics}
}

type parser struct {
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

	arena []Node

	diagnostics []Diagnostic
	depthError  bool
}

const (
	maxParseDepth  = 1000
	maxDiagnostics = 1024
)

func (p *parser) enterDepth() bool {
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

func (p *parser) exitDepth() {
	p.depth--
}

// allocNode returns a pointer to a fresh zero Node from p's arena.
func (p *parser) allocNode() *Node {
	if len(p.arena) == cap(p.arena) {
		newCap := 256
		if c := cap(p.arena); c > 0 {
			newCap = c * 2
		}
		grown := make([]Node, len(p.arena), newCap)
		copy(grown, p.arena)
		p.arena = grown
	}
	p.arena = p.arena[:len(p.arena)+1]
	return &p.arena[len(p.arena)-1]
}

func (p *parser) missingSemiOK() bool {
	return p.at(token.RBrace) || p.allowMissingTrailingSemi && p.atEnd()
}

func (p *parser) abortIfSharedAcrossBranch() {
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

func (p *parser) cur() token.Token {
	return p.toks[p.pos]
}

func (p *parser) peek(offset int) token.Token {
	idx := max(p.pos+offset, 0)
	if idx >= len(p.toks) {
		return p.toks[len(p.toks)-1]
	}
	return p.toks[idx]
}

func (p *parser) at(k token.Kind) bool {
	return p.cur().Kind == k
}

func (p *parser) atEnd() bool {
	return p.cur().Kind == token.EOF
}

func (p *parser) advance() token.Token {
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

func (p *parser) emitDiagnostic(d Diagnostic) {
	if len(p.diagnostics) >= maxDiagnostics {
		return
	}
	d.Expected = append([]token.Kind(nil), d.Expected...)
	p.diagnostics = append(p.diagnostics, d)
}

func (p *parser) emitMissingToken(expected token.Kind, context string) {
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

func (p *parser) emitMissing(code DiagnosticCode, message string, expected ...token.Kind) {
	found := p.cur()
	r := ByteRange{Start: found.Start.Offset, End: found.Start.Offset}
	p.emitDiagnostic(Diagnostic{
		Code: code, Message: message, Range: r, Found: found,
		Expected: expected, Recovery: suggestedRecovery(r),
	})
}

func (p *parser) makeRecoveryNode(start, end int, found token.Token, grammar itemGrammar, exactRemove bool) *Node {
	n := recoveryNode(p.source, start, end, found, grammar.recoveryContext, grammar.recoveryExpected)
	foundRange := tokenRange(found)
	code := DiagnosticUnexpectedToken
	recovery := suggestedRecovery(foundRange)
	if exactRemove {
		code = DiagnosticUnexpectedDelimiter
		recovery = Recovery{Kind: RecoveryRemove, Range: foundRange, Confidence: RecoveryExact}
	}
	message := n.ErrorMessage
	if message == "" {
		message = "unexpected " + strconv.Quote(found.Kind.String())
	}
	p.emitDiagnostic(Diagnostic{
		Code: code, Message: message, Range: foundRange,
		Found: found, Expected: grammar.recoveryExpected, Recovery: recovery,
	})
	return n
}

func (p *parser) recoverStuckItem(grammar itemGrammar) *Node {
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
		return nil
	}
	return p.makeRecoveryNode(start, p.toks[len(p.toks)-1].End.Offset, found, grammar, false)
}

func (p *parser) updateRecoveryLiveness(live []bool, liveFalseCount int) ([]bool, int) {
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

func (p *parser) stuckBoundary(start, startIdx int, found token.Token, grammar itemGrammar) *Node {
	if p.pos == startIdx {
		p.broken = true
		last := p.advance()
		exactRemove := found.Kind == token.RBrace || found.Kind == token.RParen || found.Kind == token.RBracket
		return p.makeRecoveryNode(start, last.End.Offset, found, grammar, exactRemove)
	}
	return p.makeRecoveryNode(start, p.toks[p.pos-1].End.Offset, found, grammar, false)
}
