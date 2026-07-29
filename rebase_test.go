package parser

import (
	"reflect"
	"testing"

	"github.com/pawnkit/pawn-parser/lexer"
)

func TestRebaseCompactTriviaMatchesCleanParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		before string
		after  string
		old    ByteRange
		next   ByteRange
	}{
		{
			name:   "replace whitespace",
			before: "stock Work() { return 1; }\nstock Keep() { return 2; }\n",
			after:  "stock Work() {    return 1; }\nstock Keep() { return 2; }\n",
			old:    ByteRange{Start: 14, End: 15},
			next:   ByteRange{Start: 14, End: 18},
		},
		{
			name:   "trailing insertion",
			before: "stock Work() { return 1; }\n",
			after:  "stock Work() { return 1; }   \n",
			old:    ByteRange{Start: 26, End: 26},
			next:   ByteRange{Start: 26, End: 29},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			before := []byte(test.before)
			after := []byte(test.after)
			previous := ParseTokensCompact(before, lexer.Tokenize(before), ParseOptions{})
			got, ok := RebaseCompactTrivia(after, lexer.Tokenize(after), previous, test.old, test.next)
			if !ok {
				t.Fatal("trivia edit was not rebased")
			}
			want := ParseTokensCompact(after, lexer.Tokenize(after), ParseOptions{})
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("rebased parse differs from clean parse:\ngot:  %#v\nwant: %#v", got, want)
			}
		})
	}
}

func TestRebaseCompactTriviaRejectsSyntaxChanges(t *testing.T) {
	t.Parallel()

	before := []byte("stock Work() { return 1; }\n")
	previous := ParseTokensCompact(before, lexer.Tokenize(before), ParseOptions{})
	tests := []struct {
		name  string
		after string
	}{
		{name: "token text", after: "stock Task() { return 1; }\n"},
		{name: "token kind", after: "stock Work() { return -1; }\n"},
		{name: "newline", after: "stock Work() {\nreturn 1; }\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, ok := RebaseCompactTrivia(
				[]byte(test.after),
				lexer.Tokenize([]byte(test.after)),
				previous,
				ByteRange{Start: 0, End: len(before)},
				ByteRange{Start: 0, End: len(test.after)},
			); ok {
				t.Fatal("syntax-changing edit was rebased")
			}
		})
	}
}

func FuzzRebaseCompactTriviaMatchesCleanParse(f *testing.F) {
	f.Add("stock Work() { return 1; }\n", uint16(15))
	f.Add("main() {\n    new value = 1;\n}\n", uint16(13))
	f.Fuzz(func(t *testing.T, text string, rawOffset uint16) {
		if len(text) == 0 || len(text) > 4096 {
			t.Skip()
		}
		before := []byte(text)
		previous := ParseTokensCompact(before, lexer.Tokenize(before), ParseOptions{})
		if previous.HasParseErrors() {
			t.Skip()
		}
		offset := int(rawOffset) % (len(before) + 1)
		after := make([]byte, 0, len(before)+1)
		after = append(after, before[:offset]...)
		after = append(after, ' ')
		after = append(after, before[offset:]...)
		got, ok := RebaseCompactTrivia(
			after,
			lexer.Tokenize(after),
			previous,
			ByteRange{Start: offset, End: offset},
			ByteRange{Start: offset, End: offset + 1},
		)
		if !ok {
			t.Skip()
		}
		want := ParseTokensCompact(after, lexer.Tokenize(after), ParseOptions{})
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("rebased parse differs from clean parse at byte %d:\ngot:  %#v\nwant: %#v", offset, got, want)
		}
	})
}
