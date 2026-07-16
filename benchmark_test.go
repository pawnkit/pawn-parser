package parser

import (
	"os"
	"testing"

	"github.com/pawnkit/pawn-parser/lexer"
)

const benchmarkFixture = realWorldFixtureDir + "/ultimate-creator/creator.pwn"

func benchmarkSource(b *testing.B) []byte {
	b.Helper()
	source, err := os.ReadFile(benchmarkFixture)
	if err != nil {
		b.Fatal(err)
	}
	return source
}

func BenchmarkParseLargeFile(b *testing.B) {
	source := benchmarkSource(b)
	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()
	for range b.N {
		if file := Parse(source); file.Root == nil {
			b.Fatal("Parse returned no tree")
		}
	}
}

func BenchmarkParseTokensLargeFile(b *testing.B) {
	source := benchmarkSource(b)
	tokens := lexer.Tokenize(source)
	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()
	for range b.N {
		if file := ParseTokens(source, tokens); file.Root == nil {
			b.Fatal("ParseTokens returned no tree")
		}
	}
}
