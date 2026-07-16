package parser

import "github.com/pawnkit/pawn-parser/token"

type compactBuildNode struct {
	leading       []token.Trivia
	trailing      []token.Trivia
	errorMessage  string
	errorExpected []token.Kind
	children      []uint32
	fields        []compactBuildField
}

type compactBuildField struct {
	id   FieldID
	node uint32
}

type compactBuilder struct {
	nodes    []compactBuildNode
	records  []CompactNode
	children compactUintArena
	fields   compactFieldArena
	trivia   parserTriviaArena
}

type compactArenaMark struct {
	blocks int
	next   int
}

type compactUintArena struct {
	blocks [][]uint32
	next   int
}

//nolint:dupl // Typed arenas intentionally share growth rules.
func (a *compactUintArena) alloc(size int) []uint32 {
	if len(a.blocks) == 0 || len(a.blocks[len(a.blocks)-1])-a.next < size {
		blockSize := 64
		if len(a.blocks) != 0 {
			blockSize = min(len(a.blocks[len(a.blocks)-1])*2, 4096)
		}
		a.blocks = append(a.blocks, make([]uint32, max(size, blockSize)))
		a.next = 0
	}
	items := a.blocks[len(a.blocks)-1][a.next : a.next+size : a.next+size]
	a.next += size
	return items
}

func (a *compactUintArena) append(items []uint32, item uint32) []uint32 {
	if len(items) == cap(items) {
		grown := a.alloc(max(4, cap(items)*2))
		copy(grown, items)
		items = grown[:len(items)]
	}
	return append(items, item)
}

func (a *compactUintArena) mark() compactArenaMark {
	return compactArenaMark{blocks: len(a.blocks), next: a.next}
}

func (a *compactUintArena) rewind(mark compactArenaMark) {
	if mark.blocks == 0 {
		a.blocks, a.next = nil, 0
		return
	}
	a.blocks = a.blocks[:mark.blocks]
	a.next = mark.next
}

type compactFieldArena struct {
	blocks [][]compactBuildField
	next   int
}

func (a *compactFieldArena) append(items []compactBuildField, item compactBuildField) []compactBuildField {
	if len(items) == cap(items) {
		capacity := max(2, cap(items)*2)
		if len(a.blocks) == 0 || len(a.blocks[len(a.blocks)-1])-a.next < capacity {
			blockSize := 32
			if len(a.blocks) != 0 {
				blockSize = min(len(a.blocks[len(a.blocks)-1])*2, 1024)
			}
			a.blocks = append(a.blocks, make([]compactBuildField, max(capacity, blockSize)))
			a.next = 0
		}
		grown := a.blocks[len(a.blocks)-1][a.next : a.next+capacity : a.next+capacity]
		a.next += capacity
		copy(grown, items)
		items = grown[:len(items)]
	}
	return append(items, item)
}

func (a *compactFieldArena) mark() compactArenaMark {
	return compactArenaMark{blocks: len(a.blocks), next: a.next}
}

func (a *compactFieldArena) rewind(mark compactArenaMark) {
	if mark.blocks == 0 {
		a.blocks, a.next = nil, 0
		return
	}
	a.blocks = a.blocks[:mark.blocks]
	a.next = mark.next
}

type compactNodeSink struct{ builder *compactBuilder }

var _ nodeSink[uint32] = compactNodeSink{}

func newCompactNodeSink(tokenCount int) compactNodeSink {
	capacity := tokenCount*2/3 + 64
	return compactNodeSink{builder: &compactBuilder{
		nodes: make([]compactBuildNode, 1, capacity), records: make([]CompactNode, 1, capacity),
	}}
}

func (compactNodeSink) Nil() uint32 { return 0 }

func (s compactNodeSink) New(kind Kind) uint32 {
	return s.Store(Node{Kind: kind})
}

func (s compactNodeSink) Store(value Node) uint32 {
	id := compactUint(len(s.builder.nodes))
	s.builder.nodes = append(s.builder.nodes, compactBuildNode{})
	s.builder.records = append(s.builder.records, CompactNode{
		Kind: value.Kind, Start: compactUint(value.Start), End: compactUint(value.End),
		TokenKind: value.Tok.Kind, TokenStart: compactUint(value.Tok.Start.Offset), TokenEnd: compactUint(value.Tok.End.Offset),
		HasError: value.HasError, MissingSemi: value.MissingSemi,
	})
	*s.node(id) = compactBuildNode{
		leading: value.Leading, trailing: value.Trailing,
		errorMessage: value.ErrorMessage, errorExpected: value.ErrorExpected,
	}
	return id
}

func (s compactNodeSink) NewLeaf(kind Kind, tok token.Token) uint32 {
	return s.Store(Node{
		Kind: kind, Tok: tok, Start: tok.Start.Offset, End: tok.End.Offset,
		Leading: tok.LeadingTrivia, Trailing: tok.TrailingTrivia,
	})
}

func (s compactNodeSink) NewNode(kind Kind, children ...uint32) uint32 {
	n := s.New(kind)
	for _, child := range children {
		s.AddChild(n, child)
	}
	if len(s.node(n).children) != 0 {
		first := s.node(n).children[0]
		s.SetStart(n, s.Start(first))
		s.SetLeading(n, s.Leading(first))
	}
	return n
}

func (s compactNodeSink) Append(nodes []uint32, node uint32) []uint32 {
	return s.builder.children.append(nodes, node)
}

func (s compactNodeSink) node(n uint32) *compactBuildNode {
	return &s.builder.nodes[n]
}

func (s compactNodeSink) record(n uint32) *CompactNode { return &s.builder.records[n] }

func (s compactNodeSink) Kind(n uint32) Kind           { return s.record(n).Kind }
func (s compactNodeSink) SetKind(n uint32, value Kind) { s.record(n).Kind = value }
func (s compactNodeSink) Token(n uint32) token.Token {
	v := s.record(n)
	return token.Token{Kind: v.TokenKind, Start: token.Position{Offset: int(v.TokenStart)}, End: token.Position{Offset: int(v.TokenEnd)}}
}

func (s compactNodeSink) SetToken(n uint32, value token.Token) {
	v := s.record(n)
	v.TokenKind, v.TokenStart, v.TokenEnd = value.Kind, compactUint(value.Start.Offset), compactUint(value.End.Offset)
}
func (s compactNodeSink) Start(n uint32) int                        { return int(s.record(n).Start) }
func (s compactNodeSink) SetStart(n uint32, value int)              { s.record(n).Start = compactUint(value) }
func (s compactNodeSink) End(n uint32) int                          { return int(s.record(n).End) }
func (s compactNodeSink) SetEnd(n uint32, value int)                { s.record(n).End = compactUint(value) }
func (s compactNodeSink) Leading(n uint32) []token.Trivia           { return s.node(n).leading }
func (s compactNodeSink) SetLeading(n uint32, value []token.Trivia) { s.node(n).leading = value }
func (s compactNodeSink) Trailing(n uint32) []token.Trivia          { return s.node(n).trailing }
func (s compactNodeSink) SetTrailing(n uint32, value []token.Trivia) {
	s.node(n).trailing = value
}
func (s compactNodeSink) HasError(n uint32) bool              { return s.record(n).HasError }
func (s compactNodeSink) SetHasError(n uint32, value bool)    { s.record(n).HasError = value }
func (s compactNodeSink) MissingSemi(n uint32) bool           { return s.record(n).MissingSemi }
func (s compactNodeSink) SetMissingSemi(n uint32, value bool) { s.record(n).MissingSemi = value }
func (compactNodeSink) Raw(uint32) []byte                     { return nil }
func (compactNodeSink) SetRaw(uint32, []byte)                 {}
func (s compactNodeSink) ErrorMessage(n uint32) string        { return s.node(n).errorMessage }
func (s compactNodeSink) SetErrorMessage(n uint32, value string) {
	s.node(n).errorMessage = value
}
func (compactNodeSink) ErrorOffset(uint32) int                { return 0 }
func (compactNodeSink) SetErrorOffset(uint32, int)            {}
func (compactNodeSink) ErrorFound(uint32) token.Kind          { return token.Invalid }
func (compactNodeSink) SetErrorFound(uint32, token.Kind)      {}
func (s compactNodeSink) ErrorExpected(n uint32) []token.Kind { return s.node(n).errorExpected }
func (s compactNodeSink) SetErrorExpected(n uint32, value []token.Kind) {
	s.node(n).errorExpected = value
}

func (s compactNodeSink) Children(n uint32) []uint32 {
	return s.node(n).children
}

func (s compactNodeSink) SetChildren(n uint32, children []uint32) {
	node := s.node(n)
	if cap(node.children) < len(children) {
		node.children = s.builder.children.alloc(len(children))[:0]
	} else {
		node.children = node.children[:0]
	}
	node.children = append(node.children, children...)
	if len(children) != 0 {
		last := children[len(children)-1]
		s.SetEnd(n, s.End(last))
		node.trailing = s.Trailing(last)
	}
}

func (s compactNodeSink) AddChild(n, child uint32) {
	if child == 0 {
		return
	}
	node := s.node(n)
	node.children = s.builder.children.append(node.children, child)
	s.SetEnd(n, s.End(child))
	node.trailing = s.Trailing(child)
	if s.HasError(child) {
		s.SetHasError(n, true)
	}
}

func (s compactNodeSink) Field(n uint32, id FieldID) uint32 {
	for _, entry := range s.node(n).fields {
		if entry.id == id {
			return entry.node
		}
	}
	return 0
}

func (s compactNodeSink) SetField(n uint32, id FieldID, child uint32) {
	if child == 0 {
		return
	}
	node := s.node(n)
	node.fields = s.builder.fields.append(node.fields, compactBuildField{id: id, node: child})
}

func (s compactNodeSink) Mark() sinkMark {
	return sinkMark{
		nodes: len(s.builder.nodes), children: s.builder.children.mark(), fields: s.builder.fields.mark(),
		trivia: s.builder.trivia.mark(),
	}
}

func (s compactNodeSink) Rewind(mark sinkMark) {
	clear(s.builder.nodes[mark.nodes:])
	s.builder.nodes = s.builder.nodes[:mark.nodes]
	s.builder.records = s.builder.records[:mark.nodes]
	s.builder.children.rewind(mark.children)
	s.builder.fields.rewind(mark.fields)
	s.builder.trivia.rewind(mark.trivia)
}

func (s compactNodeSink) AllocTrivia(size int) []token.Trivia {
	return s.builder.trivia.alloc(size)
}

func (s compactNodeSink) tree(root uint32) CompactTree {
	if root == 0 {
		return CompactTree{}
	}
	childCount, fieldCount := 0, 0
	for id := uint32(1); id < compactUint(len(s.builder.nodes)); id++ {
		node := s.node(id)
		childCount += len(node.children)
		fieldCount += len(node.fields)
	}
	tree := CompactTree{
		Nodes:    s.builder.records[1:len(s.builder.records):len(s.builder.records)],
		Children: make([]uint32, 0, childCount),
		Fields:   make([]CompactField, 0, fieldCount),
	}
	for id := uint32(1); id < compactUint(len(s.builder.nodes)); id++ {
		node := s.node(id)
		index := id - 1
		children := node.children
		childStart := compactUint(len(tree.Children))
		for _, child := range children {
			tree.Children = append(tree.Children, child-1)
		}

		fieldStart := compactUint(len(tree.Fields))
		for _, entry := range node.fields {
			tree.Fields = append(tree.Fields, CompactField{ID: entry.id, Node: entry.node - 1})
		}
		record := &tree.Nodes[index]
		record.ChildStart = childStart
		record.ChildCount = compactUint(len(children))
		record.FieldStart = fieldStart
		record.FieldCount = compactUint(len(tree.Fields)) - fieldStart
	}
	tree.Root = root - 1
	return tree
}
