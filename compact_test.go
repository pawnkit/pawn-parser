package parser

import (
	"reflect"
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
	root := compactFile.Tree.Nodes[compactFile.Tree.Root]
	if got := root.Bytes(source); string(got) != string(source) {
		t.Fatalf("compact root bytes = %q", got)
	}
	if got := root.Range(); got != (ByteRange{Start: 0, End: len(source)}) {
		t.Fatalf("compact root range = %+v", got)
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
	assertEquivalentNodes(t, pointerFile.Root, ParseCompact(source, ParseOptions{}).Expand().Root)
}

func TestCompactTreeContainsOnlyReachableNodes(t *testing.T) {
	t.Parallel()
	source := []byte("enum Color { Red, Green }\nforward Float:GetValue(Float:value);\n")
	pointer := Parse(source)
	compact := ParseForLinter(source)

	pointerCount := 0
	var countPointer func(*Node)
	countPointer = func(node *Node) {
		pointerCount++
		for _, child := range node.Children {
			countPointer(child)
		}
	}
	countPointer(pointer.Root)
	if len(compact.Tree.Nodes) != pointerCount {
		t.Fatalf("compact nodes = %d, reachable pointer nodes = %d", len(compact.Tree.Nodes), pointerCount)
	}
	if cap(compact.Tree.Nodes) != len(compact.Tree.Nodes) {
		t.Fatal("compact tree retained spare node capacity")
	}
	tokens := lexer.Tokenize(source)
	sink := newCompactNodeSink(len(tokens))
	internal := &parser[uint32, compactNodeSink]{source: source, toks: newParserTokens(tokens), sink: sink}
	internal.parseSourceFile()
	if allocated := len(sink.builder.nodes) - 1; allocated != pointerCount {
		t.Fatalf("compact arena nodes = %d, reachable pointer nodes = %d", allocated, pointerCount)
	}

	reached := make([]bool, len(compact.Tree.Nodes))
	stack := []uint32{compact.Tree.Root}
	for len(stack) != 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node >= compactUint(len(reached)) || reached[node] {
			continue
		}
		reached[node] = true
		stack = append(stack, compact.Tree.ChildIndices(node)...)
	}
	for node, ok := range reached {
		if !ok {
			t.Fatalf("compact node %d is unreachable", node)
		}
	}
	assertEquivalentNodes(t, pointer.Root, ParseCompact(source, ParseOptions{}).Expand().Root)
}

func TestParseCompactDiagnosticsMatchPointerParser(t *testing.T) {
	t.Parallel()
	for _, source := range [][]byte{
		[]byte("main( { return;"),
		[]byte("new value = ;"),
		[]byte("#if defined VALUE\nmain() {}\n"),
	} {
		pointer := Parse(source)
		compact := ParseCompact(source, ParseOptions{})
		if pointer.Broken != compact.Broken || !reflect.DeepEqual(pointer.Diagnostics, compact.Diagnostics) {
			t.Fatalf("compact diagnostics differ for %q", source)
		}
		assertEquivalentNodes(t, pointer.Root, compact.Expand().Root)
	}
}

func TestParseForLinterDiscardsTokensAndTrivia(t *testing.T) {
	t.Parallel()
	file := ParseForLinter([]byte("main() {}\n"))
	if len(file.Tree.Nodes) == 0 || file.Tokens != nil || file.Trivia != nil {
		t.Fatal("ParseForLinter retained token data")
	}
}

func TestParseCompactLexerRetentionMatchesTokenConversion(t *testing.T) {
	t.Parallel()
	source := []byte("// lead\nmain() { return value; } // tail\n")
	wantTokens, wantTrivia, wantOrigins, wantMacros := compactTokens(lexer.Tokenize(source), ParseOptions{})
	got := ParseCompact(source, ParseOptions{})
	if !reflect.DeepEqual(got.Tokens, wantTokens) {
		t.Fatalf("native compact tokens differ:\n got %#v\nwant %#v", got.Tokens, wantTokens)
	}
	if !reflect.DeepEqual(got.Trivia, wantTrivia) || !reflect.DeepEqual(got.Origins, wantOrigins) ||
		!reflect.DeepEqual(got.MacroNames, wantMacros) {
		t.Fatal("native compact lexer metadata differs from token conversion")
	}
}

func assertEquivalentNodes(t *testing.T, want, got *Node) {
	t.Helper()
	if want == nil || got == nil {
		if want != got {
			t.Fatal("expanded compact tree has a missing node")
		}
		return
	}
	if !equivalentNodeHeader(want, got) {
		t.Fatalf("expanded %s node differs", want.Kind)
	}
	if len(want.Children) != len(got.Children) {
		t.Fatalf("expanded %s children = %d, want %d", want.Kind, len(got.Children), len(want.Children))
	}
	for i := range want.Children {
		assertEquivalentNodes(t, want.Children[i], got.Children[i])
	}
	for _, name := range fieldNames {
		wantField, gotField := want.Field(name), got.Field(name)
		if (wantField == nil) != (gotField == nil) || wantField != nil && wantField.Kind != gotField.Kind {
			t.Fatalf("expanded %s field %q differs", want.Kind, name)
		}
	}
}

func equivalentNodeHeader(want, got *Node) bool {
	return want.Kind == got.Kind && want.Start == got.Start && want.End == got.End &&
		want.Tok.Kind == got.Tok.Kind && want.Tok.Start == got.Tok.Start && want.Tok.End == got.Tok.End &&
		want.HasError == got.HasError && want.MissingSemi == got.MissingSemi &&
		string(want.Raw) == string(got.Raw) && want.ErrorMessage == got.ErrorMessage &&
		want.ErrorOffset == got.ErrorOffset && want.ErrorFound == got.ErrorFound &&
		reflect.DeepEqual(want.ErrorExpected, got.ErrorExpected)
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

	expanded := file.Expand()
	if expanded.Tokens[0].Origin == nil || expanded.Tokens[0].Origin.Macro != "VALUE" {
		t.Fatal("expanded token origin was not preserved")
	}
	if expanded.Tokens[0].Origin.Parent == nil || expanded.Tokens[0].Origin.Parent.Span.File != 1 {
		t.Fatal("expanded parent origin was not preserved")
	}
}
