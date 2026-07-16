package parser

import (
	"sort"

	"github.com/pawnkit/pawn-parser/token"
)

// SyntaxNode is a lightweight handle into an immutable compact tree.
type SyntaxNode struct {
	file *CompactFile
	id   uint32
}

// Syntax returns the root syntax node.
func (f *CompactFile) Syntax() SyntaxNode {
	if f == nil || f.Tree.Root >= uint32(len(f.Tree.Nodes)) { // #nosec G115 -- Compact indexes are uint32.
		return SyntaxNode{}
	}
	return SyntaxNode{file: f, id: f.Tree.Root}
}

// Valid reports whether n refers to a tree node.
func (n SyntaxNode) Valid() bool {
	return n.file != nil && n.id < uint32(len(n.file.Tree.Nodes)) // #nosec G115 -- Compact indexes are uint32.
}

func (n SyntaxNode) compact() CompactNode {
	if !n.Valid() {
		return CompactNode{}
	}
	return n.file.Tree.Nodes[n.id]
}

// Kind returns the node kind.
func (n SyntaxNode) Kind() Kind { return n.compact().Kind }

// Range returns the node source range.
func (n SyntaxNode) Range() ByteRange { return n.compact().Range() }

// Bytes returns the node source bytes without allocating.
func (n SyntaxNode) Bytes() []byte {
	if !n.Valid() {
		return nil
	}
	return n.compact().Bytes(n.file.Source)
}

// Text returns the node source text.
func (n SyntaxNode) Text() string { return string(n.Bytes()) }

// HasError reports whether the node contains syntax errors.
func (n SyntaxNode) HasError() bool { return n.compact().HasError }

// MissingSemicolon reports whether recovery inserted a semicolon.
func (n SyntaxNode) MissingSemicolon() bool { return n.compact().MissingSemi }

// Token returns the node's primary token reference.
func (n SyntaxNode) Token() SyntaxToken {
	compact := n.compact()
	return SyntaxToken{file: n.file, kind: compact.TokenKind, start: compact.TokenStart, end: compact.TokenEnd}
}

// Field returns a named child.
func (n SyntaxNode) Field(name string) (SyntaxNode, bool) {
	if !n.Valid() {
		return SyntaxNode{}, false
	}
	id, ok := n.file.Tree.Field(n.id, name)
	return SyntaxNode{file: n.file, id: id}, ok
}

func (n SyntaxNode) field(id FieldID) (SyntaxNode, bool) {
	if !n.Valid() {
		return SyntaxNode{}, false
	}
	compact := n.compact()
	for _, field := range n.file.Tree.Fields[compact.FieldStart : compact.FieldStart+compact.FieldCount] {
		if field.ID == id {
			return SyntaxNode{file: n.file, id: field.Node}, true
		}
	}
	return SyntaxNode{}, false
}

// Children returns an allocation-free child iterator.
func (n SyntaxNode) Children() SyntaxIterator {
	if !n.Valid() {
		return SyntaxIterator{}
	}
	compact := n.compact()
	return SyntaxIterator{file: n.file, ids: n.file.Tree.Children[compact.ChildStart : compact.ChildStart+compact.ChildCount]}
}

// SyntaxIterator iterates syntax nodes without allocating.
type SyntaxIterator struct {
	file    *CompactFile
	ids     []uint32
	current uint32
	next    int
}

// Next advances the iterator.
func (i *SyntaxIterator) Next() bool {
	if i == nil || i.next >= len(i.ids) {
		return false
	}
	i.current = i.ids[i.next]
	i.next++
	return true
}

// Node returns the current node.
func (i *SyntaxIterator) Node() SyntaxNode {
	if i == nil || i.next == 0 || i.next > len(i.ids) {
		return SyntaxNode{}
	}
	return SyntaxNode{file: i.file, id: i.current}
}

// SyntaxToken is a lightweight token reference.
type SyntaxToken struct {
	file       *CompactFile
	kind       token.Kind
	start, end uint32
}

// Valid reports whether the token reference is present.
func (t SyntaxToken) Valid() bool { return t.file != nil && t.kind != token.Invalid }

// Kind returns the token kind.
func (t SyntaxToken) Kind() token.Kind { return t.kind }

// Range returns the token source range.
func (t SyntaxToken) Range() ByteRange { return ByteRange{Start: int(t.start), End: int(t.end)} }

// Bytes returns token source bytes without allocating.
func (t SyntaxToken) Bytes() []byte {
	if !t.Valid() || t.end > uint32(len(t.file.Source)) || t.start > t.end { // #nosec G115 -- Compact offsets are uint32.
		return nil
	}
	return t.file.Source[t.start:t.end]
}

// Text returns token source text.
func (t SyntaxToken) Text() string { return string(t.Bytes()) }

func (t SyntaxToken) compact() (CompactToken, bool) {
	if !t.Valid() || len(t.file.Tokens) == 0 {
		return CompactToken{}, false
	}
	index := sort.Search(len(t.file.Tokens), func(i int) bool {
		return t.file.Tokens[i].Start.Offset >= t.start
	})
	for index < len(t.file.Tokens) && t.file.Tokens[index].Start.Offset == t.start {
		candidate := t.file.Tokens[index]
		if candidate.Kind == t.kind && candidate.End.Offset == t.end {
			return candidate, true
		}
		index++
	}
	return CompactToken{}, false
}

// LeadingTrivia returns retained leading trivia.
func (t SyntaxToken) LeadingTrivia() []CompactTrivia {
	compact, ok := t.compact()
	if !ok {
		return nil
	}
	return t.trivia(compact.LeadingStart, compact.LeadingCount)
}

// TrailingTrivia returns retained trailing trivia.
func (t SyntaxToken) TrailingTrivia() []CompactTrivia {
	compact, ok := t.compact()
	if !ok {
		return nil
	}
	return t.trivia(compact.TrailingStart, compact.TrailingCount)
}

func (t SyntaxToken) trivia(start, count uint32) []CompactTrivia {
	end := start + count
	if end < start || end > uint32(len(t.file.Trivia)) { // #nosec G115 -- Compact indexes are uint32.
		return nil
	}
	return t.file.Trivia[start:end:end]
}

// Origin returns the first retained token origin.
func (t SyntaxToken) Origin() (SyntaxOrigin, bool) {
	compact, ok := t.compact()
	if !ok || compact.Origin == 0 || compact.Origin >= uint32(len(t.file.Origins)) { // #nosec G115 -- Compact indexes are uint32.
		return SyntaxOrigin{}, false
	}
	return SyntaxOrigin{file: t.file, id: compact.Origin}, true
}

// SyntaxOrigin is a lightweight origin-chain handle.
type SyntaxOrigin struct {
	file *CompactFile
	id   uint32
}

// Valid reports whether the origin exists.
func (o SyntaxOrigin) Valid() bool {
	return o.file != nil && o.id != 0 && o.id < uint32(len(o.file.Origins)) // #nosec G115 -- Compact indexes are uint32.
}

// Span returns the origin source span.
func (o SyntaxOrigin) Span() token.Span {
	if !o.Valid() {
		return token.Span{}
	}
	origin := o.file.Origins[o.id]
	return token.Span{File: origin.File, Start: expandPosition(origin.Start), End: expandPosition(origin.End)}
}

// Macro returns the originating macro name.
func (o SyntaxOrigin) Macro() string {
	if !o.Valid() {
		return ""
	}
	macro := o.file.Origins[o.id].Macro
	if macro >= uint32(len(o.file.MacroNames)) { // #nosec G115 -- Compact indexes are uint32.
		return ""
	}
	return o.file.MacroNames[macro]
}

// Parent returns the parent origin when present.
func (o SyntaxOrigin) Parent() (SyntaxOrigin, bool) {
	if !o.Valid() {
		return SyntaxOrigin{}, false
	}
	parent := o.file.Origins[o.id].Parent
	if parent == 0 || parent >= uint32(len(o.file.Origins)) { // #nosec G115 -- Compact indexes are uint32.
		return SyntaxOrigin{}, false
	}
	return SyntaxOrigin{file: o.file, id: parent}, true
}
