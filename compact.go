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
	Errors   []CompactError
	Expected []token.Kind
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
	HasRaw      bool
}

// CompactError stores sparse recovery data.
type CompactError struct {
	Message       string
	Node          uint32
	Offset        uint32
	Found         token.Kind
	ExpectedStart uint32
	ExpectedCount uint32
}

// Text returns the node's exact source text.
func (n CompactNode) Text(source []byte) string {
	return string(n.Bytes(source))
}

// Bytes returns the node's exact source bytes without allocating.
func (n CompactNode) Bytes(source []byte) []byte {
	if n.End > compactUint(len(source)) || n.Start > n.End {
		return nil
	}
	return source[int(n.Start):int(n.End)]
}

// Range returns the node's half-open source byte range.
func (n CompactNode) Range() ByteRange {
	return ByteRange{Start: int(n.Start), End: int(n.End)}
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
	if options.DiscardTokens {
		return parseTokensCompact(source, lexer.Tokenize(source), options, nil, nil)
	}
	toks, compact, trivia := lexer.TokenizeCompact(source, !options.DiscardTrivia)
	return parseTokensCompact(source, toks, options, compact, trivia)
}

// ParseTokensCompact parses tokens into a compact CST.
func ParseTokensCompact(source []byte, toks []token.Token, options ParseOptions) *CompactFile {
	return parseTokensCompact(source, toks, options, nil, nil)
}

// ParseForLinter parses source without retaining tokens or trivia.
func ParseForLinter(source []byte) *CompactFile {
	return ParseCompact(source, ParseOptions{DiscardTokens: true, DiscardTrivia: true})
}

// ParseTokensForLinter parses an existing token stream for linting.
func ParseTokensForLinter(source []byte, toks []token.Token) *CompactFile {
	return ParseTokensCompact(source, toks, ParseOptions{DiscardTokens: true, DiscardTrivia: true})
}

func parseTokensCompact(
	source []byte,
	toks []token.Token,
	options ParseOptions,
	retainedTokens []CompactToken,
	retainedTrivia []CompactTrivia,
) *CompactFile {
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
	tokens, trivia := retainedTokens, retainedTrivia
	origins, macroNames := []CompactOrigin{{}}, []string{""}
	if tokens == nil && !options.DiscardTokens {
		tokens, trivia, origins, macroNames = compactTokens(toks, options)
	}
	return &CompactFile{
		Source: source, Tokens: tokens, Trivia: trivia, Origins: origins, MacroNames: macroNames,
		Tree: sink.tree(root), Broken: p.broken, Diagnostics: p.diagnostics,
	}
}

// CompactToken stores one token with indexed metadata.
type CompactToken = token.CompactToken

// CompactTrivia stores one trivia span.
type CompactTrivia = token.CompactTrivia

// CompactPosition stores a compact source position.
type CompactPosition = token.CompactPosition

// CompactOrigin stores one origin link.
type CompactOrigin = token.CompactOrigin

func compactTokens(tokens []token.Token, options ParseOptions) ([]CompactToken, []CompactTrivia, []CompactOrigin, []string) {
	if options.DiscardTokens {
		return nil, nil, nil, nil
	}
	compact := make([]CompactToken, len(tokens))
	triviaCount := 0
	if !options.DiscardTrivia {
		for i := range tokens {
			triviaCount += len(tokens[i].LeadingTrivia) + len(tokens[i].TrailingTrivia)
		}
	}
	trivia := make([]CompactTrivia, 0, triviaCount)
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
