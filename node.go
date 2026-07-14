package parser

import (
	"strconv"
	"strings"

	"github.com/pawnkit/pawn-parser/token"
)

// Node is a single element of the concrete syntax tree produced by Parse.
type Node struct {
	Kind     Kind
	Tok      token.Token
	Children []*Node

	fields []fieldEntry

	Start int
	End   int

	HasError bool
	Raw      []byte

	ErrorMessage  string
	ErrorOffset   int
	ErrorFound    token.Kind
	ErrorExpected []token.Kind

	MissingSemi bool

	Leading  []token.Trivia
	Trailing []token.Trivia
}

type fieldEntry struct {
	name string
	node *Node
}

// Field looks up a named child. Returns nil if absent.
func (n *Node) Field(name string) *Node {
	if n == nil {
		return nil
	}
	for _, f := range n.fields {
		if f.name == name {
			return f.node
		}
	}
	return nil
}

// Text returns the node's exact source text.
func (n *Node) Text(source []byte) string {
	if n == nil || n.Start < 0 || n.End > len(source) || n.Start > n.End {
		return ""
	}
	return string(source[n.Start:n.End])
}

// LeadingTrivia returns the trivia attached before the node's first token.
func (n *Node) LeadingTrivia() []token.Trivia {
	if n == nil {
		return nil
	}
	return n.Leading
}

// TrailingTrivia returns the trivia attached after the node's last token.
func (n *Node) TrailingTrivia() []token.Trivia {
	if n == nil {
		return nil
	}
	return n.Trailing
}

// OperatorTokenHasComment reports whether n's operator token has a leading
// or trailing comment attached.
func (n *Node) OperatorTokenHasComment() bool {
	if n == nil {
		return false
	}
	for _, tr := range n.Tok.LeadingTrivia {
		if tr.Kind == token.Comment {
			return true
		}
	}
	for _, tr := range n.Tok.TrailingTrivia {
		if tr.Kind == token.Comment {
			return true
		}
	}
	return false
}

func setField(n *Node, name string, child *Node) {
	if child == nil {
		return
	}
	n.fields = append(n.fields, fieldEntry{name, child})
}

func (p *parser) newLeaf(kind Kind, tok token.Token) *Node {
	n := p.allocNode()
	n.Kind = kind
	n.Tok = tok
	n.Start = tok.Start.Offset
	n.End = tok.End.Offset
	n.Leading = tok.LeadingTrivia
	n.Trailing = tok.TrailingTrivia
	return n
}

func (p *parser) newNode(kind Kind, children ...*Node) *Node {
	n := p.allocNode()
	n.Kind = kind
	for _, c := range children {
		if c != nil {
			n.Children = append(n.Children, c)
		}
	}
	n.recalc()
	return n
}

func (n *Node) recalc() {
	if len(n.Children) == 0 {
		return
	}
	first, last := n.Children[0], n.Children[len(n.Children)-1]
	n.Start = first.Start
	n.End = last.End
	n.Leading = first.Leading
	n.Trailing = last.Trailing
	for _, c := range n.Children {
		if c.HasError {
			n.HasError = true
		}
	}
}

func (n *Node) addChild(c *Node) {
	if c == nil {
		return
	}
	n.Children = append(n.Children, c)
	n.End = c.End
	n.Trailing = c.Trailing
	if c.HasError {
		n.HasError = true
	}
}

func clampRange(source []byte, start, end int) (int, int) {
	if start < 0 {
		start = 0
	}
	if end > len(source) {
		end = len(source)
	}
	if end < start {
		end = start
	}
	return start, end
}

func rawNode(source []byte, start, end int) *Node {
	start, end = clampRange(source, start, end)
	return &Node{Kind: KindRaw, Start: start, End: end, HasError: true, Raw: source[start:end]}
}

func recoveryNode(source []byte, start, end int, found token.Token, context string, expected []token.Kind) *Node {
	n := rawNode(source, start, end)
	n.ErrorOffset = found.Start.Offset
	n.ErrorFound = found.Kind
	n.ErrorExpected = append([]token.Kind(nil), expected...)
	foundText := strconv.Quote(found.Text(source))
	if found.Kind == token.EOF {
		foundText = "end of file"
	}
	n.ErrorMessage = "unexpected " + foundText
	if context != "" {
		n.ErrorMessage += " while parsing " + context
	}
	if len(expected) > 0 {
		formatted := make([]string, len(expected))
		for i, kind := range expected {
			formatted[i] = strconv.Quote(kind.String())
		}
		n.ErrorMessage += "; expected " + strings.Join(formatted, " or ")
	}
	return n
}

func directiveSpan(source []byte, kind Kind, start, end int, leading, trailing []token.Trivia) *Node {
	start, end = clampRange(source, start, end)
	return &Node{Kind: kind, Start: start, End: end, Raw: source[start:end], Leading: leading, Trailing: trailing}
}
