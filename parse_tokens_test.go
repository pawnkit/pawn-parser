package parser

import (
	"testing"

	"github.com/pawnkit/pawn-parser/lexer"
	"github.com/pawnkit/pawn-parser/token"
)

func TestParseTokensAddsEOFPreservingCallerTokens(t *testing.T) {
	t.Parallel()

	source := []byte("main() {}")
	tokens := lexer.Tokenize(source)
	tokens = tokens[:len(tokens)-1]
	originalLen := len(tokens)
	origin := &token.Origin{
		Span: token.Span{
			File:  1,
			Start: tokens[0].Start,
			End:   tokens[0].End,
		},
	}
	tokens[0].Origin = origin

	file := ParseTokens(source, tokens)

	if file.HasParseErrors() {
		t.Fatal("ParseTokens returned parse errors")
	}
	if got := len(tokens); got != originalLen {
		t.Fatalf("caller token count = %d, want %d", got, originalLen)
	}
	if got := file.Tokens[len(file.Tokens)-1]; got.Kind != token.EOF || got.Start.Offset != len(source) || got.End.Offset != len(source) {
		t.Fatalf("appended EOF = %#v", got)
	}
	if file.Tokens[0].Origin != origin {
		t.Fatal("token origin was not preserved")
	}
}

func TestParseTokensAcceptsEmptyInput(t *testing.T) {
	t.Parallel()

	file := ParseTokens(nil, nil)
	if file.HasParseErrors() {
		t.Fatal("ParseTokens returned parse errors for empty input")
	}
	if len(file.Tokens) != 1 || file.Tokens[0].Kind != token.EOF {
		t.Fatalf("tokens = %#v, want one EOF", file.Tokens)
	}
}
