package parser

import (
	"bytes"
	"testing"

	"github.com/pawnkit/pawn-parser/lexer"
)

func TestReparseCompactDeclarationMatchesCleanParse(t *testing.T) {
	t.Parallel()

	before := []byte("stock First() { return 1; }\nstock Work() { return value; }\nstock Last() { return 3; }\n")
	offset := bytes.Index(before, []byte("value"))
	after := append([]byte(nil), before...)
	after = append(after[:offset], append([]byte("(value)"), after[offset+len("value"):]...)...)

	previous := ParseTokensCompact(before, lexer.Tokenize(before), ParseOptions{})
	got, ok := ReparseCompactDeclaration(
		after,
		lexer.Tokenize(after),
		previous,
		ByteRange{Start: offset, End: offset + len("value")},
		ByteRange{Start: offset, End: offset + len("(value)")},
	)
	if !ok {
		t.Fatal("declaration edit was not reparsed")
	}
	want := ParseTokensCompact(after, lexer.Tokenize(after), ParseOptions{})
	assertEquivalentNodes(t, want.Expand().Root, got.Expand().Root)
	if !bytes.Equal(got.Source, want.Source) || len(got.Tokens) != len(want.Tokens) {
		t.Fatal("reparsed file did not retain the current source and tokens")
	}
}

func TestReparseCompactDeclarationRebasesCaseMacroHint(t *testing.T) {
	t.Parallel()

	before := []byte("stock First() { return 1; }\nmain() { switch (0) { case HASH<Next page>: return 1; } }\n")
	after := bytes.Replace(before, []byte("switch (0)"), []byte("switch (10)"), 1)
	options := func(text []byte) ParseOptions {
		const invocation = "HASH<Next page>"
		start := bytes.Index(text, []byte(invocation))
		return ParseOptions{CaseMacroEnds: map[int]int{start: start + len(invocation)}}
	}
	previous := ParseCompact(before, options(before))
	if previous.HasParseErrors() {
		t.Fatalf("case macro did not parse: %+v", previous.Diagnostics)
	}
	offset := bytes.Index(before, []byte("0)"))
	got, ok := ReparseCompactDeclarationWithOptions(after, lexer.Tokenize(after), previous,
		ByteRange{Start: offset, End: offset + 1},
		ByteRange{Start: offset, End: offset + 2}, options(after),
	)
	if !ok {
		t.Fatal("case macro declaration was not reparsed")
	}
	want := ParseCompact(after, options(after))
	assertEquivalentNodes(t, want.Expand().Root, got.Expand().Root)
}

func TestReparseCompactDeclarationRejectsBoundaryEdit(t *testing.T) {
	t.Parallel()

	before := []byte("stock Work() { return 1; }\nstock Last() { return 2; }\n")
	previous := ParseTokensCompact(before, lexer.Tokenize(before), ParseOptions{})
	after := append([]byte("const Value = 1;\n"), before...)
	if _, ok := ReparseCompactDeclaration(
		after,
		lexer.Tokenize(after),
		previous,
		ByteRange{},
		ByteRange{End: len("const Value = 1;\n")},
	); ok {
		t.Fatal("top-level insertion was reparsed as one declaration")
	}
}
