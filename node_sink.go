package parser

import "github.com/pawnkit/pawn-parser/token"

// nodeSink supplies mutable parser nodes.
type nodeSink[N comparable] interface {
	Nil() N
	New(Kind) N
	Store(Node) N
	NewLeaf(Kind, token.Token) N
	NewNode(Kind, ...N) N
	Append([]N, N) []N

	Kind(N) Kind
	SetKind(N, Kind)
	Token(N) token.Token
	SetToken(N, token.Token)
	Start(N) int
	SetStart(N, int)
	End(N) int
	SetEnd(N, int)
	Leading(N) []token.Trivia
	SetLeading(N, []token.Trivia)
	Trailing(N) []token.Trivia
	SetTrailing(N, []token.Trivia)
	HasError(N) bool
	SetHasError(N, bool)
	MissingSemi(N) bool
	SetMissingSemi(N, bool)
	Raw(N) []byte
	SetRaw(N, []byte)
	ErrorMessage(N) string
	SetErrorMessage(N, string)
	ErrorOffset(N) int
	SetErrorOffset(N, int)
	ErrorFound(N) token.Kind
	SetErrorFound(N, token.Kind)
	ErrorExpected(N) []token.Kind
	SetErrorExpected(N, []token.Kind)

	Children(N) []N
	SetChildren(N, []N)
	AddChild(N, N)
	Field(N, FieldID) N
	SetField(N, FieldID, N)

	Mark() sinkMark
	Rewind(sinkMark)
	AllocTrivia(int) []token.Trivia
}

type sinkMark struct {
	storage    parserStorageMark
	nodes      int
	edgeCount  int
	fieldCount int
	errors     int
	children   compactArenaMark
	fields     compactArenaMark
	trivia     nodeArenaMark
}

type pointerNodeSink struct {
	storage *parserStorage
	source  []byte
}

var _ nodeSink[*Node] = pointerNodeSink{}

func (s pointerNodeSink) Nil() *Node { return nil }

func (s pointerNodeSink) New(kind Kind) *Node {
	n := s.storage.arena.alloc()
	n.Kind = kind
	return n
}

func (s pointerNodeSink) Store(value Node) *Node {
	n := s.storage.arena.alloc()
	*n = value
	return n
}

func (s pointerNodeSink) NewLeaf(kind Kind, tok token.Token) *Node {
	return newPointerLeaf(s.storage, kind, tok)
}

func (s pointerNodeSink) NewNode(kind Kind, children ...*Node) *Node {
	return newPointerNode(s.storage, kind, children...)
}

func (s pointerNodeSink) Append(nodes []*Node, node *Node) []*Node {
	if len(nodes) == cap(nodes) {
		capacity := max(4, cap(nodes)*2)
		grown := s.storage.children.alloc(capacity)
		copy(grown, nodes)
		nodes = grown[:len(nodes)]
	}
	return append(nodes, node)
}

func (pointerNodeSink) Kind(n *Node) Kind                         { return n.Kind }
func (pointerNodeSink) SetKind(n *Node, value Kind)               { n.Kind = value }
func (pointerNodeSink) Token(n *Node) token.Token                 { return n.Tok }
func (pointerNodeSink) SetToken(n *Node, value token.Token)       { n.Tok = value }
func (pointerNodeSink) Start(n *Node) int                         { return n.Start }
func (pointerNodeSink) SetStart(n *Node, value int)               { n.Start = value }
func (pointerNodeSink) End(n *Node) int                           { return n.End }
func (pointerNodeSink) SetEnd(n *Node, value int)                 { n.End = value }
func (pointerNodeSink) Leading(n *Node) []token.Trivia            { return n.Leading }
func (pointerNodeSink) SetLeading(n *Node, value []token.Trivia)  { n.Leading = value }
func (pointerNodeSink) Trailing(n *Node) []token.Trivia           { return n.Trailing }
func (pointerNodeSink) SetTrailing(n *Node, value []token.Trivia) { n.Trailing = value }
func (pointerNodeSink) HasError(n *Node) bool                     { return n.HasError }
func (pointerNodeSink) SetHasError(n *Node, value bool)           { n.HasError = value }
func (pointerNodeSink) MissingSemi(n *Node) bool                  { return n.MissingSemi }
func (pointerNodeSink) SetMissingSemi(n *Node, value bool)        { n.MissingSemi = value }
func (pointerNodeSink) Raw(n *Node) []byte                        { return n.Raw }
func (pointerNodeSink) SetRaw(n *Node, value []byte)              { n.Raw = value }
func (pointerNodeSink) ErrorMessage(n *Node) string               { return n.ErrorMessage }
func (pointerNodeSink) SetErrorMessage(n *Node, value string)     { n.ErrorMessage = value }
func (pointerNodeSink) ErrorOffset(n *Node) int                   { return n.ErrorOffset }
func (pointerNodeSink) SetErrorOffset(n *Node, value int)         { n.ErrorOffset = value }
func (pointerNodeSink) ErrorFound(n *Node) token.Kind             { return n.ErrorFound }
func (pointerNodeSink) SetErrorFound(n *Node, value token.Kind)   { n.ErrorFound = value }
func (pointerNodeSink) ErrorExpected(n *Node) []token.Kind        { return n.ErrorExpected }
func (pointerNodeSink) SetErrorExpected(n *Node, value []token.Kind) {
	n.ErrorExpected = value
}
func (pointerNodeSink) Children(n *Node) []*Node           { return n.Children }
func (pointerNodeSink) SetChildren(n *Node, value []*Node) { n.Children = value }
func (s pointerNodeSink) AddChild(n, child *Node)          { addPointerChild(s.storage, n, child) }
func (pointerNodeSink) Field(n *Node, id FieldID) *Node    { return n.field(id) }
func (s pointerNodeSink) SetField(n *Node, id FieldID, child *Node) {
	setPointerField(s.storage, n, id, child)
}

func (s pointerNodeSink) Mark() sinkMark {
	return sinkMark{storage: s.storage.mark()}
}

func (s pointerNodeSink) Rewind(mark sinkMark) {
	s.storage.rewind(mark.storage)
}

func (s pointerNodeSink) AllocTrivia(size int) []token.Trivia {
	return s.storage.trivia.alloc(size)
}
