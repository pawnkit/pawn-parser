// Package lexer is a standalone tokenizer for the Pawn language.
package lexer

import "github.com/pawnkit/pawn-parser/token"

// RawTokens tokenizes src without attaching trivia (whitespace/comments)
// to the returned tokens.
func RawTokens(src []byte) []token.Token {
	s := newScanner(src)
	out := make([]token.Token, 0, len(src)/3)
	for {
		r := s.nextRaw()
		out = append(out, token.Token{Kind: r.kind, Start: r.start, End: r.end})
		if r.kind == token.EOF {
			break
		}
	}
	return out
}

// Tokenize tokenizes src, attaching leading and trailing trivia (whitespace/comments)
// to each token.
func Tokenize(src []byte) []token.Token {
	s := newScanner(src)
	tokens := make([]token.Token, 0, len(src)/3)
	var leading []token.Trivia

	for {
		r := s.nextRaw()
		if r.kind.IsTrivia() {
			leading = append(leading, token.Trivia{Kind: r.kind, Start: r.start, End: r.end})
			continue
		}

		tok := token.Token{Kind: r.kind, Start: r.start, End: r.end, LeadingTrivia: leading}
		leading = nil

		if r.kind == token.EOF {
			tokens = append(tokens, tok)
			break
		}

		var trailing []token.Trivia
		for {
			save := *s
			r2 := s.nextRaw()
			if !r2.kind.IsTrivia() {
				*s = save
				break
			}
			trailing = append(trailing, token.Trivia{Kind: r2.kind, Start: r2.start, End: r2.end})
			if r2.kind == token.Newline {
				break
			}
		}
		tok.TrailingTrivia = trailing
		tokens = append(tokens, tok)
	}

	return tokens
}
