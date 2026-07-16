package parser_test

import (
	"testing"

	"github.com/pawnkit/pawn-parser"
)

func TestPublicParserAPI(t *testing.T) {
	t.Parallel()
	source := []byte("stock Example(value) { return value + 1; }")
	file := parser.Parse(source)
	if file.HasParseErrors() {
		t.Fatal("valid source was marked broken")
	}
	if file.Root == nil || len(file.Tokens) == 0 {
		t.Fatal("public parse result did not expose its CST and tokens")
	}
	first := file.Tokens[0]
	if first.Start.Offset != 0 {
		t.Fatalf("unexpected first token offset: %d", first.Start.Offset)
	}
	if got := file.Root.Text(source); got != string(source) {
		t.Fatalf("root source text mismatch: %q", got)
	}
	if got := file.Root.Bytes(source); string(got) != string(source) {
		t.Fatalf("root source bytes mismatch: %q", got)
	}
	if got := file.Root.Range(); got != (parser.ByteRange{Start: 0, End: len(source)}) {
		t.Fatalf("root range mismatch: %+v", got)
	}
}
