package parser

import (
	"sort"

	"github.com/pawnkit/pawn-parser/token"
)

// Expand builds the pointer CST represented by f.
func (f *CompactFile) Expand() *File {
	return f.ExpandWithOptions(ParseOptions{})
}

// ExpandWithOptions builds the pointer CST represented by f.
func (f *CompactFile) ExpandWithOptions(options ParseOptions) *File {
	return f.ExpandTokensWithOptions(nil, options)
}

// ExpandTokensWithOptions builds the pointer CST with its original tokens.
func (f *CompactFile) ExpandTokensWithOptions(tokens []token.Token, options ParseOptions) *File {
	if f == nil {
		return nil
	}
	storage := new(parserStorage)
	if tokens == nil {
		tokens = f.expandTokens(storage, options.DiscardTrivia)
	} else {
		tokens = retainedTokens(tokens, options)
	}
	nodes := make([]*Node, len(f.Tree.Nodes))
	for i, compact := range f.Tree.Nodes {
		tok := expandedNodeToken(tokens, compact)
		nodes[i] = storage.arena.alloc()
		*nodes[i] = Node{
			Kind: compact.Kind, Tok: tok, Start: int(compact.Start), End: int(compact.End),
			HasError: compact.HasError, MissingSemi: compact.MissingSemi,
			Leading: tok.LeadingTrivia, Trailing: tok.TrailingTrivia,
		}
		if compact.HasRaw && compact.Start <= compact.End && compact.End <= uint32(len(f.Source)) { // #nosec G115 -- Compact offsets are uint32.
			nodes[i].Raw = f.Source[compact.Start:compact.End]
		}
	}
	for i, compact := range f.Tree.Nodes {
		node := nodes[i]
		if compact.ChildCount != 0 {
			node.Children = storage.children.alloc(int(compact.ChildCount))[:0]
			for _, child := range f.Tree.Children[compact.ChildStart : compact.ChildStart+compact.ChildCount] {
				node.Children = append(node.Children, nodes[child])
			}
		}
		if len(node.Children) != 0 {
			node.Leading = node.Children[0].Leading
			node.Trailing = node.Children[len(node.Children)-1].Trailing
		}
		for _, field := range f.Tree.Fields[compact.FieldStart : compact.FieldStart+compact.FieldCount] {
			setPointerField(storage, node, field.ID, nodes[field.Node])
		}
	}
	for _, compact := range f.Tree.Errors {
		if compact.Node >= uint32(len(nodes)) { // #nosec G115 -- Compact indexes are uint32.
			continue
		}
		node := nodes[compact.Node]
		node.ErrorMessage = compact.Message
		node.ErrorOffset = int(compact.Offset)
		node.ErrorFound = compact.Found
		end := compact.ExpectedStart + compact.ExpectedCount
		if end >= compact.ExpectedStart && end <= uint32(len(f.Tree.Expected)) { // #nosec G115 -- Compact indexes are uint32.
			node.ErrorExpected = append([]token.Kind(nil), f.Tree.Expected[compact.ExpectedStart:end]...)
		}
	}
	var root *Node
	if f.Tree.Root < uint32(len(nodes)) { // #nosec G115 -- Compact indexes are uint32.
		root = nodes[f.Tree.Root]
	}
	if options.DiscardTokens {
		tokens = nil
	}
	return &File{
		Source: f.Source, Tokens: tokens, Root: root, Broken: f.Broken,
		Diagnostics: append([]Diagnostic(nil), f.Diagnostics...),
	}
}

func expandedNodeToken(tokens []token.Token, node CompactNode) token.Token {
	start := int(node.TokenStart)
	index := sort.Search(len(tokens), func(i int) bool {
		return tokens[i].Start.Offset >= start
	})
	for index < len(tokens) && tokens[index].Start.Offset == start {
		item := tokens[index]
		if item.Kind == node.TokenKind && item.End.Offset == int(node.TokenEnd) {
			return item
		}
		index++
	}
	return token.Token{}
}

func (f *CompactFile) expandTokens(storage *parserStorage, discardTrivia bool) []token.Token {
	origins := make([]*token.Origin, len(f.Origins))
	for i := 1; i < len(origins); i++ {
		origins[i] = new(token.Origin)
	}
	for i := 1; i < len(origins); i++ {
		compact := f.Origins[i]
		origin := origins[i]
		origin.Span = token.Span{
			File: compact.File, Start: expandPosition(compact.Start), End: expandPosition(compact.End),
		}
		if compact.Macro < uint32(len(f.MacroNames)) { // #nosec G115 -- Compact indexes are uint32.
			origin.Macro = f.MacroNames[compact.Macro]
		}
		if compact.Parent < uint32(len(origins)) { // #nosec G115 -- Compact indexes are uint32.
			origin.Parent = origins[compact.Parent]
		}
	}
	tokens := make([]token.Token, len(f.Tokens))
	for i, compact := range f.Tokens {
		tokens[i] = token.Token{
			Kind: compact.Kind, Start: expandPosition(compact.Start), End: expandPosition(compact.End),
		}
		if !discardTrivia {
			tokens[i].LeadingTrivia = f.expandTrivia(storage, compact.LeadingStart, compact.LeadingCount)
			tokens[i].TrailingTrivia = f.expandTrivia(storage, compact.TrailingStart, compact.TrailingCount)
		}
		if compact.Origin < uint32(len(origins)) { // #nosec G115 -- Compact indexes are uint32.
			tokens[i].Origin = origins[compact.Origin]
		}
	}
	return tokens
}

func (f *CompactFile) expandTrivia(storage *parserStorage, start, count uint32) []token.Trivia {
	end := start + count
	if end < start || end > uint32(len(f.Trivia)) { // #nosec G115 -- Compact indexes are uint32.
		return nil
	}
	trivia := storage.trivia.alloc(int(count))
	for i, compact := range f.Trivia[start:end] {
		trivia[i] = token.Trivia{
			Kind: compact.Kind, Start: expandPosition(compact.Start), End: expandPosition(compact.End),
		}
	}
	return trivia
}

func expandPosition(position CompactPosition) token.Position {
	return token.Position{Offset: int(position.Offset), Line: int(position.Line), Col: int(position.Col)}
}
