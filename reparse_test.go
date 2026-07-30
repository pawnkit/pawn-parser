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
