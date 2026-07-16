package parser

import (
	"os"
	"strings"
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

func BenchmarkParseGenericArguments(b *testing.B) {
	var source strings.Builder
	source.WriteString("main() { Call(")
	for i := range 2000 {
		if i != 0 {
			source.WriteByte(',')
		}
		source.WriteString("Type<Value>")
	}
	source.WriteString("); }")
	data := []byte(source.String())
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for range b.N {
		if file := ParseForLinter(data); file.HasParseErrors() {
			b.Fatal("generic argument source did not parse")
		}
	}
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

func BenchmarkParseForLinterLargeFile(b *testing.B) {
	source := benchmarkSource(b)
	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()
	for range b.N {
		file := ParseForLinter(source)
		if len(file.Tree.Nodes) == 0 {
			b.Fatal("ParseForLinter returned no tree")
		}
	}
}

func BenchmarkParseCompactLargeFile(b *testing.B) {
	source := benchmarkSource(b)
	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()
	for range b.N {
		file := ParseCompact(source, ParseOptions{DiscardTokens: true, DiscardTrivia: true})
		if len(file.Tree.Nodes) == 0 {
			b.Fatal("ParseCompact returned no tree")
		}
	}
}

func BenchmarkParseCompactRetainedLargeFile(b *testing.B) {
	source := benchmarkSource(b)
	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()
	for range b.N {
		file := ParseCompact(source, ParseOptions{})
		if len(file.Tree.Nodes) == 0 || len(file.Tokens) == 0 {
			b.Fatal("ParseCompact returned incomplete syntax")
		}
	}
}

func BenchmarkTokensOnlyLargeFile(b *testing.B) {
	source := benchmarkSource(b)
	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()
	for range b.N {
		file := ParseWithProfile(source, ProfileTokensOnly)
		if len(file.Tokens) == 0 || len(file.Tree.Nodes) != 0 {
			b.Fatal("tokens-only profile returned invalid output")
		}
	}
}
