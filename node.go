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

	fieldData *nodeFieldData

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
	id   FieldID
	node *Node
}

type nodeFieldData struct {
	inline [2]fieldEntry
	count  int
	spill  []fieldEntry
}

// Field looks up a named child. Returns nil if absent.
func (n *Node) Field(name string) *Node {
	if n == nil {
		return nil
	}
	return n.field(lookupFieldID(name))
}

func (n *Node) field(id FieldID) *Node {
	if n.fieldData == nil {
		return nil
	}
	for _, f := range n.fieldData.inline[:min(n.fieldData.count, len(n.fieldData.inline))] {
		if f.id == id {
			return f.node
		}
	}
	for _, f := range n.fieldData.spill {
		if f.id == id {
			return f.node
		}
	}
	return nil
}

// Text returns the node's exact source text.
func (n *Node) Text(source []byte) string {
	return string(n.Bytes(source))
}

// Bytes returns the node's exact source bytes without allocating.
func (n *Node) Bytes(source []byte) []byte {
	if n == nil || n.Start < 0 || n.End > len(source) || n.Start > n.End {
		return nil
	}
	return source[n.Start:n.End]
}

// Range returns the node's half-open source byte range.
func (n *Node) Range() ByteRange {
	if n == nil {
		return ByteRange{}
	}
	return ByteRange{Start: n.Start, End: n.End}
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

func setPointerField(storage *parserStorage, n *Node, id FieldID, child *Node) {
	if child == nil {
		return
	}
	if n.fieldData == nil {
		n.fieldData = storage.fields.alloc()
	}
	entry := fieldEntry{id, child}
	if n.fieldData.count < len(n.fieldData.inline) {
		n.fieldData.inline[n.fieldData.count] = entry
	} else {
		if len(n.fieldData.spill) == cap(n.fieldData.spill) {
			capacity := max(2, cap(n.fieldData.spill)*2)
			spill := storage.entries.alloc(capacity)
			copy(spill, n.fieldData.spill)
			n.fieldData.spill = spill[:len(n.fieldData.spill)]
		}
		n.fieldData.spill = append(n.fieldData.spill, entry)
	}
	n.fieldData.count++
}

func newPointerLeaf(storage *parserStorage, kind Kind, tok token.Token) *Node {
	n := storage.arena.alloc()
	n.Kind = kind
	n.Tok = tok
	n.Start = tok.Start.Offset
	n.End = tok.End.Offset
	n.Leading = tok.LeadingTrivia
	n.Trailing = tok.TrailingTrivia
	return n
}

func newPointerNode(storage *parserStorage, kind Kind, children ...*Node) *Node {
	n := storage.arena.alloc()
	n.Kind = kind
	count := 0
	for _, c := range children {
		if c != nil {
			count++
		}
	}
	if count != 0 {
		n.Children = storage.children.alloc(count)[:0]
		for _, c := range children {
			if c != nil {
				n.Children = append(n.Children, c)
			}
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

func addPointerChild(storage *parserStorage, n, c *Node) {
	if c == nil {
		return
	}
	if len(n.Children) == cap(n.Children) {
		capacity := max(4, cap(n.Children)*2)
		children := storage.children.alloc(capacity)
		copy(children, n.Children)
		n.Children = children[:len(n.Children)]
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

func (p *parser[N, S]) recoveryNode(start, end int, found token.Token, context string, expected []token.Kind) N {
	start, end = clampRange(p.source, start, end)
	n := p.sink.Store(Node{Kind: KindRaw, Start: start, End: end, HasError: true, Raw: p.source[start:end]})
	p.sink.SetErrorOffset(n, found.Start.Offset)
	p.sink.SetErrorFound(n, found.Kind)
	p.sink.SetErrorExpected(n, append([]token.Kind(nil), expected...))
	foundText := strconv.Quote(found.Text(p.source))
	if found.Kind == token.EOF {
		foundText = "end of file"
	}
	message := "unexpected " + foundText
	if context != "" {
		message += " while parsing " + context
	}
	if len(expected) > 0 {
		formatted := make([]string, len(expected))
		for i, kind := range expected {
			formatted[i] = strconv.Quote(kind.String())
		}
		message += "; expected " + strings.Join(formatted, " or ")
	}
	p.sink.SetErrorMessage(n, message)
	return n
}

func (p *parser[N, S]) directiveSpan(kind Kind, start, end int, leading, trailing []token.Trivia) N {
	start, end = clampRange(p.source, start, end)
	return p.sink.Store(Node{Kind: kind, Start: start, End: end, Raw: p.source[start:end], Leading: leading, Trailing: trailing})
}
