// Package lexer is a standalone tokenizer for the Pawn language.
package lexer

import (
	"sync"

	"github.com/pawnkit/pawn-parser/token"
)

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
	blocks []*builtTokenBlock
	next   int
	count  int
}

type (
	builtTokenBlock struct{ data []builtToken }
	triviaBlock     struct{ data []token.Trivia }
)

const builderBlockLevels = 5

var (
	builtTokenBlockPools [builderBlockLevels]sync.Pool
	triviaBlockPools     [builderBlockLevels]sync.Pool
)

func builderBlockLevel(size int) int {
	level := 0
	for size > 256 && level < builderBlockLevels-1 {
		size /= 2
		level++
	}
	return level
}

func acquireBuiltTokenBlock(size int) *builtTokenBlock {
	if block, ok := builtTokenBlockPools[builderBlockLevel(size)].Get().(*builtTokenBlock); ok && cap(block.data) >= size {
		block.data = block.data[:size]
		return block
	}
	return &builtTokenBlock{data: make([]builtToken, size)}
}

func releaseBuiltTokenBlock(block *builtTokenBlock) {
	builtTokenBlockPools[builderBlockLevel(len(block.data))].Put(block)
}

func acquireTriviaBlock(size int) *triviaBlock {
	if block, ok := triviaBlockPools[builderBlockLevel(size)].Get().(*triviaBlock); ok && cap(block.data) >= size {
		block.data = block.data[:size]
		return block
	}
	return &triviaBlock{data: make([]token.Trivia, size)}
}

func releaseTriviaBlock(block *triviaBlock) {
	triviaBlockPools[builderBlockLevel(len(block.data))].Put(block)
}

func nextBuilderBlockSize(blockCount, lastSize int) int {
	if blockCount == 0 {
		return 256
	}
	return min(lastSize*2, 4096)
}

type triviaBuilder struct {
	blocks []*triviaBlock
	next   int
	count  int
}

func (b *triviaBuilder) append(value token.Trivia) { //nolint:dupl // Keep the hot path concrete.
	if len(b.blocks) == 0 || b.next == len(b.blocks[len(b.blocks)-1].data) {
		lastSize := 0
		if len(b.blocks) != 0 {
			lastSize = len(b.blocks[len(b.blocks)-1].data)
		}
		size := nextBuilderBlockSize(len(b.blocks), lastSize)
		b.blocks = append(b.blocks, acquireTriviaBlock(size))
		b.next = 0
	}
	b.blocks[len(b.blocks)-1].data[b.next] = value
	b.next++
	b.count++
}

func (b *triviaBuilder) finish() []token.Trivia {
	trivia := make([]token.Trivia, b.count)
	output := 0
	for blockIndex, block := range b.blocks {
		data := block.data
		if blockIndex == len(b.blocks)-1 {
			data = data[:b.next]
		}
		output += copy(trivia[output:], data)
		releaseTriviaBlock(block)
	}
	return trivia
}

func (b *tokenBuilder) append(value builtToken) { //nolint:dupl // Keep the hot path concrete.
	if len(b.blocks) == 0 || b.next == len(b.blocks[len(b.blocks)-1].data) {
		lastSize := 0
		if len(b.blocks) != 0 {
			lastSize = len(b.blocks[len(b.blocks)-1].data)
		}
		size := nextBuilderBlockSize(len(b.blocks), lastSize)
		b.blocks = append(b.blocks, acquireBuiltTokenBlock(size))
		b.next = 0
	}
	b.blocks[len(b.blocks)-1].data[b.next] = value
	b.next++
	b.count++
}

func (b *tokenBuilder) finish(trivia []token.Trivia) []token.Token {
	tokens := make([]token.Token, b.count)
	output := 0
	for blockIndex, block := range b.blocks {
		data := block.data
		if blockIndex == len(b.blocks)-1 {
			data = data[:b.next]
		}
		for _, built := range data {
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
		releaseBuiltTokenBlock(block)
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
