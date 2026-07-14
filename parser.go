// Package parser is a reusable concrete-syntax-tree parser for the Pawn
// language used by SA-MP and open.mp projects.
package parser

import (
	"github.com/pawnkit/pawn-parser/lexer"
	"github.com/pawnkit/pawn-parser/token"
)

// File is the result of parsing one Pawn source file/buffer.
type File struct {
	Source []byte
	Tokens []token.Token
	Root   *Node

	Broken bool
}

// HasParseErrors reports whether f is nil, has no root node, or was marked
// broken during parsing.
func (f *File) HasParseErrors() bool {
	return f == nil || f.Root == nil || f.Broken
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
	return &File{Source: source, Tokens: toks, Root: root, Broken: p.broken}
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
}

const maxParseDepth = 1000

func (p *parser) enterDepth() bool {
	p.depth++
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

func (p *parser) recoverStuckItem(preserveSemicolon bool) *Node {
	start := p.cur().Start.Offset
	startIdx := p.pos
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
			if preserveSemicolon {
				return p.stuckBoundary(start, startIdx)
			}
			last := p.advance()
			return rawNode(p.source, start, last.End.Offset)
		case token.RBrace, token.RParen, token.RBracket:
			if depth > 0 {
				if counting {
					depth--
				}
				p.advance()
				continue
			}
			return p.stuckBoundary(start, startIdx)
		case token.Hash:
			if depth > 0 {
				p.advance()
				continue
			}
			return p.stuckBoundary(start, startIdx)
		default:
			p.advance()
		}
	}
	if p.pos == startIdx {
		p.broken = true
		return nil
	}
	return rawNode(p.source, start, p.toks[len(p.toks)-1].End.Offset)
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

func (p *parser) stuckBoundary(start, startIdx int) *Node {
	if p.pos == startIdx {
		p.broken = true
		last := p.advance()
		return rawNode(p.source, start, last.End.Offset)
	}
	return rawNode(p.source, start, p.toks[p.pos-1].End.Offset)
}
