package parser

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

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

//nolint:paralleltest // Measures process-local resource use.
func TestCompactParserPerformanceBudget(t *testing.T) {
	if os.Getenv("PAWNKIT_PERFORMANCE_BUDGET") == "" {
		t.Skip("set PAWNKIT_PERFORMANCE_BUDGET to run performance budgets")
	}
	source, err := os.ReadFile(benchmarkFixture)
	if err != nil {
		t.Fatal(err)
	}
	parse := func() {
		file := ParseCompact(source, ParseOptions{DiscardTokens: true, DiscardTrivia: true})
		if len(file.Tree.Nodes) == 0 {
			t.Fatal("ParseCompact returned no tree")
		}
	}
	parse()
	allocations := testing.AllocsPerRun(3, parse)
	start := time.Now()
	parse()
	elapsed := time.Since(start)
	if allocations > 120_000 {
		t.Fatalf("ParseCompact allocations = %.0f, budget = 120000", allocations)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("ParseCompact duration = %s, budget = 500ms", elapsed)
	}
}

func BenchmarkParseCompactExternalFile(b *testing.B) {
	path := os.Getenv("PAWN_PARSER_BENCH_FILE")
	if path == "" {
		b.Skip()
	}
	source, err := os.ReadFile(path) //nolint:gosec // Developer-supplied benchmark path.
	if err != nil {
		b.Fatal(err)
	}
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

func BenchmarkRebaseCompactTriviaLargeFile(b *testing.B) {
	before := benchmarkSource(b)
	start := bytes.IndexByte(before, ' ')
	if start < 0 {
		b.Fatal("fixture has no whitespace")
	}
	after := make([]byte, 0, len(before)+1)
	after = append(after, before[:start]...)
	after = append(after, ' ')
	after = append(after, before[start:]...)
	previous := ParseTokensCompact(before, lexer.Tokenize(before), ParseOptions{})
	tokens := lexer.Tokenize(after)

	b.ReportAllocs()
	b.SetBytes(int64(len(after)))
	b.ResetTimer()
	for range b.N {
		file, ok := RebaseCompactTrivia(
			after,
			tokens,
			previous,
			ByteRange{Start: start, End: start},
			ByteRange{Start: start, End: start + 1},
		)
		if !ok || len(file.Tree.Nodes) == 0 {
			b.Fatal("RebaseCompactTrivia returned no tree")
		}
	}
}

func BenchmarkReparseCompactDeclarationLargeFile(b *testing.B) {
	before := benchmarkSource(b)
	start := bytes.LastIndex(before, []byte("return"))
	if start < 0 {
		b.Fatal("fixture has no return statement")
	}
	start += len("return ")
	after := make([]byte, 0, len(before)+2)
	after = append(after, before[:start]...)
	after = append(after, '(')
	after = append(after, before[start:]...)
	end := bytes.IndexByte(after[start+1:], ';')
	if end < 0 {
		b.Fatal("fixture return has no semicolon")
	}
	end += start + 1
	after = append(after[:end], append([]byte{')'}, after[end:]...)...)
	previous := ParseTokensCompact(before, lexer.Tokenize(before), ParseOptions{})
	tokens := lexer.Tokenize(after)

	b.ReportAllocs()
	b.SetBytes(int64(len(after)))
	b.ResetTimer()
	for range b.N {
		file, ok := ReparseCompactDeclaration(
			after,
			tokens,
			previous,
			ByteRange{Start: start, End: end - 1},
			ByteRange{Start: start, End: end + 1},
		)
		if !ok || len(file.Tree.Nodes) == 0 {
			b.Fatal("ReparseCompactDeclaration returned no tree")
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

func BenchmarkTypedSyntaxTraversal(b *testing.B) {
	file := ParseWithProfile(benchmarkSource(b), ProfileAnalysis)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		count := 0
		declarations := file.Syntax().Declarations()
		for declarations.Next() {
			if function, ok := AsFunction(declarations.Declaration()); ok {
				parameters := function.Parameters()
				for parameters.Next() {
					count++
				}
			}
		}
		if count == 0 {
			b.Fatal("typed traversal found no parameters")
		}
	}
}

func BenchmarkDeclarationIndexLargeFile(b *testing.B) {
	file := ParseWithProfile(benchmarkSource(b), ProfileAnalysis)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		index := BuildDeclarationIndex(file)
		if index.Len() == 0 {
			b.Fatal("declaration index is empty")
		}
	}
}
