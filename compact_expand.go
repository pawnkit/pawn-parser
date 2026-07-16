package parser

import "github.com/pawnkit/pawn-parser/token"

// Expand builds the pointer CST represented by f.
func (f *CompactFile) Expand() *File {
	if f == nil {
		return nil
	}
	tokens := f.expandTokens()
	nodes := make([]*Node, len(f.Tree.Nodes))
	tokenBySpan := make(map[compactTokenKey]token.Token, len(tokens))
	for _, tok := range tokens {
		tokenBySpan[compactTokenKey{tok.Kind, tok.Start.Offset, tok.End.Offset}] = tok
	}
	for i, compact := range f.Tree.Nodes {
		tok := tokenBySpan[compactTokenKey{compact.TokenKind, int(compact.TokenStart), int(compact.TokenEnd)}]
		nodes[i] = &Node{
			Kind: compact.Kind, Tok: tok, Start: int(compact.Start), End: int(compact.End),
			HasError: compact.HasError, MissingSemi: compact.MissingSemi,
			Leading: tok.LeadingTrivia, Trailing: tok.TrailingTrivia,
		}
	}
	for i, compact := range f.Tree.Nodes {
		node := nodes[i]
		for _, child := range f.Tree.Children[compact.ChildStart : compact.ChildStart+compact.ChildCount] {
			node.Children = append(node.Children, nodes[child])
		}
		if len(node.Children) != 0 {
			node.Leading = node.Children[0].Leading
			node.Trailing = node.Children[len(node.Children)-1].Trailing
		}
		for _, field := range f.Tree.Fields[compact.FieldStart : compact.FieldStart+compact.FieldCount] {
			setExpandedField(node, field.ID, nodes[field.Node])
		}
	}
	var root *Node
	if f.Tree.Root < uint32(len(nodes)) { // #nosec G115 -- Compact indexes are uint32.
		root = nodes[f.Tree.Root]
	}
	return &File{
		Source: f.Source, Tokens: tokens, Root: root, Broken: f.Broken,
		Diagnostics: append([]Diagnostic(nil), f.Diagnostics...),
	}
}

type compactTokenKey struct {
	kind       token.Kind
	start, end int
}

func (f *CompactFile) expandTokens() []token.Token {
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
			LeadingTrivia:  f.expandTrivia(compact.LeadingStart, compact.LeadingCount),
			TrailingTrivia: f.expandTrivia(compact.TrailingStart, compact.TrailingCount),
		}
		if compact.Origin < uint32(len(origins)) { // #nosec G115 -- Compact indexes are uint32.
			tokens[i].Origin = origins[compact.Origin]
		}
	}
	return tokens
}

func (f *CompactFile) expandTrivia(start, count uint32) []token.Trivia {
	end := start + count
	if end < start || end > uint32(len(f.Trivia)) { // #nosec G115 -- Compact indexes are uint32.
		return nil
	}
	trivia := make([]token.Trivia, count)
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

func setExpandedField(node *Node, id FieldID, child *Node) {
	if node.fieldData == nil {
		node.fieldData = new(nodeFieldData)
	}
	entry := fieldEntry{id: id, node: child}
	if node.fieldData.count < len(node.fieldData.inline) {
		node.fieldData.inline[node.fieldData.count] = entry
	} else {
		node.fieldData.spill = append(node.fieldData.spill, entry)
	}
	node.fieldData.count++
}
