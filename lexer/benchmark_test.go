package lexer

import (
	"bytes"
	"testing"
)

func BenchmarkTokenizeTrivia(b *testing.B) {
	line := []byte("new value = Call(other); // comment\n")
	source := bytes.Repeat(line, 10_000)
	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()
	for range b.N {
		if tokens := Tokenize(source); len(tokens) == 0 {
			b.Fatal("Tokenize returned no tokens")
		}
	}
}
