package parser

import (
	"math"
	"sort"

	"github.com/pawnkit/pawn-parser/lexer"
	"github.com/pawnkit/pawn-parser/token"
)

// CompactFile is an index-based CST.
type CompactFile struct {
	Source      []byte
	Tokens      []CompactToken
	Trivia      []CompactTrivia
	Origins     []CompactOrigin
	MacroNames  []string
	Tree        CompactTree
	Broken      bool
	Diagnostics []Diagnostic
}

// HasParseErrors reports whether compact parsing produced a syntax error.
func (f *CompactFile) HasParseErrors() bool {
	return f == nil || len(f.Tree.Nodes) == 0 || f.Broken ||
		f.Tree.Nodes[f.Tree.Root].HasError || len(f.Diagnostics) != 0
}

// CompactTree stores nodes and edges in arrays.
type CompactTree struct {
	Nodes    []CompactNode
	Children []uint32
	Fields   []CompactField
	Root     uint32
}

// CompactNode stores one tree node.
type CompactNode struct {
	Kind Kind

	Start uint32
	End   uint32

	TokenKind  token.Kind
	TokenStart uint32
	TokenEnd   uint32

	ChildStart uint32
	ChildCount uint32
	FieldStart uint32
	FieldCount uint32

	HasError    bool
	MissingSemi bool
}

// Text returns the node's exact source text.
func (n CompactNode) Text(source []byte) string {
	if n.End > compactUint(len(source)) || n.Start > n.End {
		return ""
	}
	return string(source[int(n.Start):int(n.End)])
}

// CompactField maps a field name to a node.
type CompactField struct {
	ID   FieldID
	Node uint32
}

// ChildIndices returns the child indices for node.
func (t *CompactTree) ChildIndices(node uint32) []uint32 {
	if t == nil || node >= compactUint(len(t.Nodes)) {
		return nil
	}
	n := t.Nodes[node]
	return t.Children[n.ChildStart : n.ChildStart+n.ChildCount]
}

// Field returns the node index associated with name.
func (t *CompactTree) Field(node uint32, name string) (uint32, bool) {
	if t == nil || node >= compactUint(len(t.Nodes)) {
		return 0, false
	}
	id := lookupFieldID(name)
	n := t.Nodes[node]
	for _, field := range t.Fields[n.FieldStart : n.FieldStart+n.FieldCount] {
		if field.ID == id {
			return field.Node, true
		}
	}
	return 0, false
}

// ParseCompact parses source into a compact CST.
func ParseCompact(source []byte, options ParseOptions) *CompactFile {
	return parseTokensCompact(source, lexer.Tokenize(source), options)
}

// ParseTokensCompact parses tokens into a compact CST.
func ParseTokensCompact(source []byte, toks []token.Token, options ParseOptions) *CompactFile {
	return parseTokensCompact(source, toks, options)
}

func parseTokensCompact(source []byte, toks []token.Token, options ParseOptions) *CompactFile {
	if len(toks) == 0 || toks[len(toks)-1].Kind != token.EOF {
		end := token.Position{Offset: len(source)}
		toks = append(append([]token.Token(nil), toks...), token.Token{Kind: token.EOF, Start: end, End: end})
	}
	sink := newCompactNodeSink(len(toks))
	p := &parser[uint32, compactNodeSink]{
		source: source, toks: toks, sink: sink,
	}
	root := p.parseSourceFile()
	p.buildDiagnosticCoverage()
	p.ensureErrorDiagnostics(root)
	sort.SliceStable(p.diagnostics, func(i, j int) bool {
		if p.diagnostics[i].Range.Start != p.diagnostics[j].Range.Start {
			return p.diagnostics[i].Range.Start < p.diagnostics[j].Range.Start
		}
		return p.diagnostics[i].Range.End < p.diagnostics[j].Range.End
	})
	tokens, trivia, origins, macroNames := compactTokens(toks, options)
	return &CompactFile{
		Source: source, Tokens: tokens, Trivia: trivia, Origins: origins, MacroNames: macroNames,
		Tree: sink.tree(root), Broken: p.broken, Diagnostics: p.diagnostics,
	}
}

// CompactToken stores one token.
type CompactToken struct {
	Kind token.Kind

	Start CompactPosition
	End   CompactPosition

	LeadingStart  uint32
	LeadingCount  uint32
	TrailingStart uint32
	TrailingCount uint32
	Origin        uint32
}

// CompactTrivia stores one trivia span.
type CompactTrivia struct {
	Kind  token.Kind
	Start CompactPosition
	End   CompactPosition
}

// CompactPosition stores a source position.
type CompactPosition struct {
	Offset uint32
	Line   uint32
	Col    uint32
}

// CompactOrigin stores one origin link.
type CompactOrigin struct {
	File   uint32
	Start  CompactPosition
	End    CompactPosition
	Macro  uint32
	Parent uint32
}

func compactTokens(tokens []token.Token, options ParseOptions) ([]CompactToken, []CompactTrivia, []CompactOrigin, []string) {
	if options.DiscardTokens {
		return nil, nil, nil, nil
	}
	compact := make([]CompactToken, len(tokens))
	var trivia []CompactTrivia
	origins := []CompactOrigin{{}}
	macroNames := []string{""}
	originIDs := make(map[*token.Origin]uint32)
	macroIDs := make(map[string]uint32)

	var addOrigin func(*token.Origin) uint32
	addOrigin = func(origin *token.Origin) uint32 {
		if origin == nil {
			return 0
		}
		if id, ok := originIDs[origin]; ok {
			return id
		}
		macro := uint32(0)
		if origin.Macro != "" {
			var ok bool
			macro, ok = macroIDs[origin.Macro]
			if !ok {
				macro = compactUint(len(macroNames))
				macroIDs[origin.Macro] = macro
				macroNames = append(macroNames, origin.Macro)
			}
		}
		id := compactUint(len(origins))
		originIDs[origin] = id
		origins = append(origins, CompactOrigin{})
		origins[id] = CompactOrigin{
			File: origin.Span.File, Start: compactPosition(origin.Span.Start),
			End: compactPosition(origin.Span.End), Macro: macro,
			Parent: addOrigin(origin.Parent),
		}
		return id
	}

	addTrivia := func(items []token.Trivia) (uint32, uint32) {
		if options.DiscardTrivia || len(items) == 0 {
			return 0, 0
		}
		start := compactUint(len(trivia))
		for _, item := range items {
			trivia = append(trivia, CompactTrivia{
				Kind: item.Kind, Start: compactPosition(item.Start), End: compactPosition(item.End),
			})
		}
		return start, compactUint(len(items))
	}

	for i, item := range tokens {
		leadingStart, leadingCount := addTrivia(item.LeadingTrivia)
		trailingStart, trailingCount := addTrivia(item.TrailingTrivia)
		compact[i] = CompactToken{
			Kind: item.Kind, Start: compactPosition(item.Start), End: compactPosition(item.End),
			LeadingStart: leadingStart, LeadingCount: leadingCount,
			TrailingStart: trailingStart, TrailingCount: trailingCount,
			Origin: addOrigin(item.Origin),
		}
	}
	return compact, trivia, origins, macroNames
}

func compactPosition(position token.Position) CompactPosition {
	return CompactPosition{
		Offset: compactUint(position.Offset), Line: compactUint(position.Line), Col: compactUint(position.Col),
	}
}

func compactUint(value int) uint32 {
	if value < 0 || uint64(value) > math.MaxUint32 {
		panic("compact syntax exceeds uint32")
	}
	return uint32(value) // #nosec G115 -- Bounds checked above.
}
