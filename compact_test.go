package parser

import (
	"testing"

	"github.com/pawnkit/pawn-parser/lexer"
	"github.com/pawnkit/pawn-parser/token"
)

func TestParseCompactPreservesTreeShapeAndFields(t *testing.T) {
	t.Parallel()

	source := []byte("main() { return value; }\n")
	pointerFile := Parse(source)
	compactFile := ParseCompact(source, ParseOptions{DiscardTokens: true, DiscardTrivia: true})
	if compactFile.HasParseErrors() {
		t.Fatal("ParseCompact returned parse errors")
	}
	if compactFile.Tokens != nil {
		t.Fatal("ParseCompact retained discarded tokens")
	}
	if got := compactFile.Tree.Nodes[compactFile.Tree.Root].Text(source); got != string(source) {
		t.Fatalf("compact root text = %q", got)
	}

	pointerCount := 0
	var count func(*Node)
	count = func(node *Node) {
		pointerCount++
		for _, child := range node.Children {
			count(child)
		}
	}
	count(pointerFile.Root)
	if len(compactFile.Tree.Nodes) != pointerCount {
		t.Fatalf("compact node count = %d, want %d", len(compactFile.Tree.Nodes), pointerCount)
	}

	var returnNode uint32
	for i, node := range compactFile.Tree.Nodes {
		if node.Kind == KindReturnStatement {
			returnNode = compactUint(i)
			break
		}
	}
	value, ok := compactFile.Tree.Field(returnNode, "value")
	if !ok || compactFile.Tree.Nodes[value].Text(source) != "value" {
		t.Fatal("compact return value field was not preserved")
	}
}

func TestParseTokensCompactPreservesOrigins(t *testing.T) {
	t.Parallel()

	source := []byte("value")
	tokens := lexer.Tokenize(source)
	parent := &token.Origin{
		Span: token.Span{File: 1, Start: token.Position{Offset: 3}, End: token.Position{Offset: 8}},
	}
	tokens[0].Origin = &token.Origin{
		Span:  token.Span{File: 2, Start: token.Position{Offset: 10}, End: token.Position{Offset: 15}},
		Macro: "VALUE", Parent: parent,
	}

	file := ParseTokensCompact(source, tokens, ParseOptions{})
	origin := file.Tokens[0].Origin
	if origin == 0 || file.Origins[origin].File != 2 {
		t.Fatal("token origin was not preserved")
	}
	if file.MacroNames[file.Origins[origin].Macro] != "VALUE" {
		t.Fatal("origin macro was not preserved")
	}
	parentID := file.Origins[origin].Parent
	if parentID == 0 || file.Origins[parentID].File != 1 {
		t.Fatal("parent origin was not preserved")
	}
}
