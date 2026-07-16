package lexer

import (
	"strings"
	"testing"

	"github.com/pawnkit/pawn-parser/token"
)

func TestCompactSyntaxMatchesTokens(t *testing.T) {
	t.Parallel()
	source := []byte("// lead\nnew value = Call(1); // tail\n")
	want := Tokenize(source)
	compact, lines := TokenizeSyntax(source)
	if got := lines.Position(12); got.Line != 2 || got.Col != 5 {
		t.Fatalf("position = %+v, want line 2 column 5", got)
	}
	if len(compact) != len(want) {
		t.Fatalf("compact token count = %d, want %d", len(compact), len(want))
	}
	for i, item := range compact {
		if item.Kind != want[i].Kind || int(item.Start) != want[i].Start.Offset ||
			int(item.End) != want[i].End.Offset {
			t.Fatalf("compact token %d = %+v, want %+v", i, item, want[i])
		}
		if (item.LeadingFlags&token.TriviaPresent != 0) != (len(want[i].LeadingTrivia) != 0) {
			t.Fatalf("compact token %d leading trivia summary differs", i)
		}
		endsLine := false
		for _, trivia := range want[i].TrailingTrivia {
			endsLine = endsLine || trivia.Kind == token.Newline
		}
		if (item.TrailingFlags&token.TriviaEndsLine != 0) != endsLine {
			t.Fatalf("compact token %d trailing line summary differs", i)
		}
	}
}

func kinds(toks []token.Token) []token.Kind {
	out := make([]token.Kind, 0, len(toks))
	for _, t := range toks {
		out = append(out, t.Kind)
	}
	return out
}

func assertKinds(t *testing.T, src string, want ...token.Kind) {
	t.Helper()
	toks := Tokenize([]byte(src))
	got := kinds(toks)
	if len(got) != len(want) {
		t.Fatalf("Tokenize(%q) = %v, want %v", src, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Tokenize(%q)[%d] = %v, want %v (full: %v)", src, i, got[i], want[i], got)
		}
	}
}

func TestIdentifiersAndKeywords(t *testing.T) {
	t.Parallel()
	assertKinds(t, "foo", token.Identifier, token.EOF)
	assertKinds(t, "public", token.KwPublic, token.EOF)
	assertKinds(t, "_underscore123", token.Identifier, token.EOF)
}

func TestAtSignInIdentifiers(t *testing.T) {
	t.Parallel()
	assertKinds(t, "@timer", token.Identifier, token.EOF)
	assertKinds(t, "Include_open@mp_directly", token.Identifier, token.EOF)
	assertKinds(t, "_@floatround_method", token.Identifier, token.EOF)

	src := []byte("forward @timer();")
	toks := Tokenize(src)
	if toks[1].Text(src) != "@timer" {
		t.Fatalf("expected identifier text %q, got %q", "@timer", toks[1].Text(src))
	}
}

func TestNumbers(t *testing.T) {
	t.Parallel()
	assertKinds(t, "123", token.IntLiteral, token.EOF)
	assertKinds(t, "0x1F", token.IntLiteral, token.EOF)
	assertKinds(t, "0X1F", token.IntLiteral, token.EOF)
	assertKinds(t, "0b1010", token.IntLiteral, token.EOF)
	assertKinds(t, "0B1010", token.IntLiteral, token.EOF)
	assertKinds(t, "1.5", token.FloatLiteral, token.EOF)
	assertKinds(t, "1.5e10", token.FloatLiteral, token.EOF)
	assertKinds(t, "1.5E10", token.FloatLiteral, token.EOF)
	assertKinds(t, "1e-5", token.FloatLiteral, token.EOF)
	assertKinds(t, "1e+5", token.FloatLiteral, token.EOF)
	assertKinds(t, ".5", token.FloatLiteral, token.EOF)
	assertKinds(t, "1e", token.IntLiteral, token.Identifier, token.EOF)
	assertKinds(t, "100_000", token.IntLiteral, token.EOF)
	assertKinds(t, "0xFF_FF", token.IntLiteral, token.EOF)
	assertKinds(t, "0b1010_0101", token.IntLiteral, token.EOF)
}

func TestStringsAndChars(t *testing.T) {
	t.Parallel()
	assertKinds(t, `"hello"`, token.StringLiteral, token.EOF)
	assertKinds(t, `"escaped \" quote"`, token.StringLiteral, token.EOF)
	assertKinds(t, `'c'`, token.CharLiteral, token.EOF)
	assertKinds(t, `'\n'`, token.CharLiteral, token.EOF)
	assertKinds(t, `!"packed"`, token.PackedString, token.EOF)
}

func TestUnterminatedStringDoesNotPanic(t *testing.T) {
	t.Parallel()
	toks := Tokenize([]byte(`"unterminated`))
	if toks[0].Kind != token.Unknown {
		t.Fatalf("expected Unknown for unterminated string, got %v", toks[0].Kind)
	}
}

func TestComments(t *testing.T) {
	t.Parallel()
	toks := Tokenize([]byte("foo // trailing\nbar"))
	if len(toks) != 3 {
		t.Fatalf("expected 3 significant tokens, got %d: %v", len(toks), kinds(toks))
	}
	if len(toks[0].TrailingTrivia) == 0 {
		t.Fatalf("expected trailing trivia (comment) attached to foo")
	}
	foundComment := false
	for _, tr := range toks[0].TrailingTrivia {
		if tr.Kind == token.Comment {
			foundComment = true
		}
	}
	if !foundComment {
		t.Fatalf("expected a Comment in trailing trivia, got %v", toks[0].TrailingTrivia)
	}
}

func TestLineCommentStopsBeforeCRLF(t *testing.T) {
	t.Parallel()
	src := []byte("// note\r\ncode")
	toks := Tokenize(src)
	comment := toks[0].LeadingTrivia[0]
	if comment.Kind != token.Comment {
		t.Fatalf("expected first leading trivia to be a comment, got %v", comment.Kind)
	}
	if text := comment.Text(src); text != "// note" {
		t.Fatalf("comment text = %q, want %q (no trailing \\r)", text, "// note")
	}
}

func TestBlockCommentUnterminated(t *testing.T) {
	t.Parallel()
	toks := RawTokens([]byte("/* never closes"))
	if toks[0].Kind != token.Comment {
		t.Fatalf("expected unterminated block comment to still be Comment, got %v", toks[0].Kind)
	}
}

func TestOperators(t *testing.T) {
	t.Parallel()
	assertKinds(t, "+", token.Plus, token.EOF)
	assertKinds(t, "+=", token.PlusAssign, token.EOF)
	assertKinds(t, "<<=", token.ShlAssign, token.EOF)
	assertKinds(t, "...", token.Ellipsis, token.EOF)
	assertKinds(t, "..", token.DotDot, token.EOF)
	assertKinds(t, ".", token.Dot, token.EOF)
	assertKinds(t, "&&", token.AndAnd, token.EOF)
	assertKinds(t, "&", token.Amp, token.EOF)
	assertKinds(t, "++", token.PlusPlus, token.EOF)
	assertKinds(t, ">>", token.Shr, token.EOF)
	assertKinds(t, ">>>", token.Ushr, token.EOF)
	assertKinds(t, ">>>=", token.UshrAssign, token.EOF)
	assertKinds(t, "a >>> b", token.Identifier, token.Ushr, token.Identifier, token.EOF)
}

func TestMacroParams(t *testing.T) {
	t.Parallel()
	assertKinds(t, "%0", token.MacroParam, token.EOF)
	assertKinds(t, "%1", token.MacroParam, token.EOF)
	assertKinds(t, "%%", token.MacroParam, token.EOF)
}

func TestDirectiveHash(t *testing.T) {
	t.Parallel()
	assertKinds(t, "#include", token.Hash, token.Identifier, token.EOF)
}

func TestLineContinuation(t *testing.T) {
	t.Parallel()
	toks := Tokenize([]byte("#define FOO(%0) \\\n    %0 + 1\n"))
	foundPlus := false
	for _, tok := range toks {
		if tok.Kind == token.Plus {
			foundPlus = true
		}
	}
	if !foundPlus {
		t.Fatalf("expected to lex past line continuation, got %v", kinds(toks))
	}
}

func TestTextRoundTrip(t *testing.T) {
	t.Parallel()
	src := []byte("stock Foo(playerid) { return playerid; }")
	toks := Tokenize(src)
	for _, tok := range toks {
		if tok.Kind == token.EOF {
			continue
		}
		if tok.Text(src) == "" {
			t.Fatalf("token %v has empty text", tok.Kind)
		}
	}
}

func TestNeverPanicsOnArbitraryBytes(t *testing.T) {
	t.Parallel()
	inputs := []string{
		"",
		"\x00\x01\x02",
		"\"",
		"'",
		"/*",
		"//",
		"\\",
		"#",
		"%",
		strings.Repeat("(", 5000),
		"\r\n\r\n\r\n",
		"0x",
		"1.",
		"\xff\xfe",
	}
	for _, in := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic on input %q: %v", in, r)
				}
			}()
			Tokenize([]byte(in))
		}()
	}
}

func FuzzTokenize(f *testing.F) {
	seeds := []string{
		"public OnGameModeInit() { return 1; }",
		"#define FOO(%0,%1) %0 + %1",
		"new Float:x = 0.5;",
		"\"unterminated",
		"/* unterminated",
		"'\\n'",
		"!\"packed\"",
		"forward @timer();\r\n// note\r\ncode",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic on input %q: %v", src, r)
			}
		}()
		Tokenize([]byte(src))
	})
}
