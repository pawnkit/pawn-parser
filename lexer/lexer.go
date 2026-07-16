// Package lexer is a standalone tokenizer for the Pawn language.
package lexer

import "github.com/pawnkit/pawn-parser/token"

// RawTokens tokenizes src without attaching trivia (whitespace/comments)
// to the returned tokens.
func RawTokens(src []byte) []token.Token {
	s := newScanner(src)
	out := make([]token.Token, 0, initialTokenCapacity(len(src)))
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
	var tokens tokenBuilder
	var trivia triviaBuilder
	leadingStart := 0

	for {
		r := s.nextRaw()
		if r.kind.IsTrivia() {
			trivia.append(token.Trivia{Kind: r.kind, Start: r.start, End: r.end})
			continue
		}

		attached := triviaRange{
			leadingStart:  leadingStart,
			trailingStart: trivia.count,
			trailingEnd:   trivia.count,
		}

		if r.kind == token.EOF {
			tokens.append(builtToken{kind: r.kind, start: r.start, end: r.end, trivia: attached})
			break
		}

		for {
			save := *s
			r2 := s.nextRaw()
			if !r2.kind.IsTrivia() {
				*s = save
				break
			}
			trivia.append(token.Trivia{Kind: r2.kind, Start: r2.start, End: r2.end})
			if r2.kind == token.Newline {
				break
			}
		}
		attached.trailingEnd = trivia.count
		leadingStart = trivia.count
		tokens.append(builtToken{kind: r.kind, start: r.start, end: r.end, trivia: attached})
	}

	return tokens.finish(trivia.finish())
}

const maxInitialTokenCapacity = 4096

type triviaRange struct {
	leadingStart  int
	trailingStart int
	trailingEnd   int
}

type builtToken struct {
	kind   token.Kind
	start  token.Position
	end    token.Position
	trivia triviaRange
}

type tokenBuilder struct {
	blocks [][]builtToken
	next   int
	count  int
}

type triviaBuilder struct {
	blocks [][]token.Trivia
	next   int
	count  int
}

func (b *triviaBuilder) append(value token.Trivia) {
	if len(b.blocks) == 0 || b.next == len(b.blocks[len(b.blocks)-1]) {
		size := 256
		if len(b.blocks) != 0 {
			size = min(len(b.blocks[len(b.blocks)-1])*2, 4096)
		}
		b.blocks = append(b.blocks, make([]token.Trivia, size))
		b.next = 0
	}
	b.blocks[len(b.blocks)-1][b.next] = value
	b.next++
	b.count++
}

func (b *triviaBuilder) finish() []token.Trivia {
	trivia := make([]token.Trivia, b.count)
	output := 0
	for blockIndex, block := range b.blocks {
		if blockIndex == len(b.blocks)-1 {
			block = block[:b.next]
		}
		output += copy(trivia[output:], block)
	}
	return trivia
}

func (b *tokenBuilder) append(value builtToken) {
	if len(b.blocks) == 0 || b.next == len(b.blocks[len(b.blocks)-1]) {
		size := 256
		if len(b.blocks) != 0 {
			size = min(len(b.blocks[len(b.blocks)-1])*2, 4096)
		}
		b.blocks = append(b.blocks, make([]builtToken, size))
		b.next = 0
	}
	b.blocks[len(b.blocks)-1][b.next] = value
	b.next++
	b.count++
}

func (b *tokenBuilder) finish(trivia []token.Trivia) []token.Token {
	tokens := make([]token.Token, b.count)
	output := 0
	for blockIndex, block := range b.blocks {
		if blockIndex == len(b.blocks)-1 {
			block = block[:b.next]
		}
		for _, built := range block {
			r := built.trivia
			tokens[output] = token.Token{
				Kind:           built.kind,
				Start:          built.start,
				End:            built.end,
				LeadingTrivia:  triviaSlice(trivia, r.leadingStart, r.trailingStart),
				TrailingTrivia: triviaSlice(trivia, r.trailingStart, r.trailingEnd),
			}
			output++
		}
	}
	return tokens
}

func initialTokenCapacity(sourceLen int) int {
	return min(sourceLen/8+1, maxInitialTokenCapacity)
}

func triviaSlice(trivia []token.Trivia, start, end int) []token.Trivia {
	if start == end {
		return nil
	}
	return trivia[start:end:end]
}
