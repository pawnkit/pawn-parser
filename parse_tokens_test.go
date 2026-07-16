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

func TestParseOptionsDiscardTokensAndTrivia(t *testing.T) {
	t.Parallel()

	source := []byte("// leading\nmain() { return 1; } // trailing\n")
	tokens := lexer.Tokenize(source)
	file := ParseTokensWithOptions(source, tokens, ParseOptions{
		DiscardTokens: true,
		DiscardTrivia: true,
	})
	if file.Root == nil || file.HasParseErrors() {
		t.Fatal("lightweight parse did not produce a valid tree")
	}
	if file.Tokens != nil {
		t.Fatal("lightweight parse retained tokens")
	}
	var visit func(*Node)
	visit = func(node *Node) {
		if len(node.Leading) != 0 || len(node.Trailing) != 0 ||
			len(node.Tok.LeadingTrivia) != 0 || len(node.Tok.TrailingTrivia) != 0 {
			t.Fatalf("node %s retained trivia", node.Kind)
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	visit(file.Root)
	if len(tokens) == 0 || len(tokens[0].LeadingTrivia) == 0 {
		t.Fatal("ParseTokensWithOptions modified caller-owned tokens")
	}
}
