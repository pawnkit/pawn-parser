package parser

import "testing"

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

	var returnNode int
	for i, node := range compactFile.Tree.Nodes {
		if node.Kind == KindReturnStatement {
			returnNode = i
			break
		}
	}
	value, ok := compactFile.Tree.Field(returnNode, "value")
	if !ok || compactFile.Tree.Nodes[value].Text(source) != "value" {
		t.Fatal("compact return value field was not preserved")
	}
}
