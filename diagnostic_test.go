package parser

import (
	"slices"
	"strings"
	"testing"

	"github.com/pawnkit/pawn-parser/lexer"
	"github.com/pawnkit/pawn-parser/token"
)

func TestValidInputHasNoDiagnostics(t *testing.T) {
	t.Parallel()
	f := Parse([]byte("main() { return 1; }\n"))
	if len(f.Diagnostics) != 0 {
		t.Fatalf("valid input produced diagnostics: %+v", f.Diagnostics)
	}
}

func TestMissingDelimiterExactRecoveryReparsesCleanly(t *testing.T) {
	t.Parallel()
	src := "main() { return (1; }\n"
	f := Parse([]byte(src))
	d := findDiagnostic(t, f.Diagnostics, DiagnosticMissingToken)
	if d.Range.Start != strings.Index(src, ";") || d.Range.Start != d.Range.End {
		t.Fatalf("missing delimiter has wrong range: %+v", d.Range)
	}
	if !slices.Equal(d.Expected, []token.Kind{token.RParen}) {
		t.Fatalf("missing delimiter has wrong expectation: %v", d.Expected)
	}
	if d.Recovery.Kind != RecoveryInsert || d.Recovery.Confidence != RecoveryExact || d.Recovery.Replacement != ")" {
		t.Fatalf("missing delimiter has wrong recovery: %+v", d.Recovery)
	}

	repaired := applyRecovery(t, src, d.Recovery)
	reparsed := Parse([]byte(repaired))
	if findDiagnosticOptional(reparsed.Diagnostics, DiagnosticMissingToken) != nil {
		t.Fatalf("exact recovery did not remove its diagnostic: %q: %+v", repaired, reparsed.Diagnostics)
	}
}

func TestUnexpectedClosingDelimiterHasExactRemoval(t *testing.T) {
	t.Parallel()
	src := "main() {} }\n"
	f := Parse([]byte(src))
	d := findDiagnostic(t, f.Diagnostics, DiagnosticUnexpectedDelimiter)
	if d.Found.Kind != token.RBrace || d.Recovery.Kind != RecoveryRemove || d.Recovery.Confidence != RecoveryExact {
		t.Fatalf("unexpected delimiter has wrong recovery: %+v", d)
	}
	if reparsed := Parse([]byte(applyRecovery(t, src, d.Recovery))); len(reparsed.Diagnostics) != 0 {
		t.Fatalf("removing unexpected delimiter did not produce clean input: %+v", reparsed.Diagnostics)
	}
}

func TestMissingExpressionAndIdentifierDiagnostics(t *testing.T) {
	t.Parallel()
	src := "main() { value = ; new = 1; }\n"
	f := Parse([]byte(src))
	expression := findDiagnostic(t, f.Diagnostics, DiagnosticMissingExpression)
	if expression.Range.Start != strings.Index(src, ";") || expression.Recovery.Confidence != RecoverySuggested {
		t.Fatalf("missing expression diagnostic is not narrowly suggested: %+v", expression)
	}
	identifier := findDiagnostic(t, f.Diagnostics, DiagnosticMissingIdentifier)
	if !slices.Contains(identifier.Expected, token.Identifier) {
		t.Fatalf("missing identifier expectation absent: %+v", identifier)
	}
}

func TestRawRecoveryDiagnosticsRemainOrderedAndSuggested(t *testing.T) {
	t.Parallel()
	src := "new Float:a[3][2] = { 1, 2} 3, 4} 5, 6} }\n"
	f := Parse([]byte(src))
	previous := -1
	count := 0
	for _, d := range f.Diagnostics {
		if d.Range.Start < previous {
			t.Fatalf("diagnostics out of source order: %+v", f.Diagnostics)
		}
		previous = d.Range.Start
		if d.Code == DiagnosticUnexpectedToken {
			count++
			if d.Recovery.Kind != RecoveryNone || d.Recovery.Confidence != RecoverySuggested {
				t.Fatalf("ambiguous raw recovery must only be suggested: %+v", d)
			}
		}
	}
	if count < 2 {
		t.Fatalf("expected multiple independent raw diagnostics, got %+v", f.Diagnostics)
	}
}

func TestParseTokensDiagnosticUsesVirtualOffsetAndOrigin(t *testing.T) {
	t.Parallel()
	code := "main() { value = ; }\n"
	const base = 100
	source := []byte(strings.Repeat(" ", base) + code)
	toks := lexer.Tokenize([]byte(code))
	origin := &token.Origin{Span: token.Span{File: 7, Start: token.Position{Offset: 12}, End: token.Position{Offset: 13}}, Macro: "EXPANDED"}
	for i := range toks {
		shiftToken(&toks[i], base)
		if toks[i].Kind == token.Semicolon {
			toks[i].Origin = origin
		}
	}
	f := ParseTokens(source, toks)
	d := findDiagnostic(t, f.Diagnostics, DiagnosticMissingExpression)
	want := base + strings.Index(code, ";")
	if d.Range.Start != want || d.Found.Start.Offset != want {
		t.Fatalf("diagnostic lost virtual offset: %+v", d)
	}
	if d.Found.Origin != origin || d.Found.Origin.Macro != "EXPANDED" {
		t.Fatalf("diagnostic lost token origin: %+v", d.Found.Origin)
	}
}

func TestDiagnosticsAreBounded(t *testing.T) {
	t.Parallel()
	f := Parse([]byte(strings.Repeat("}", maxDiagnostics+100)))
	if len(f.Diagnostics) > maxDiagnostics {
		t.Fatalf("diagnostic collection exceeded bound: %d", len(f.Diagnostics))
	}
}

func TestMaximumDepthProducesDiagnostic(t *testing.T) {
	t.Parallel()
	src := "main() { return " + strings.Repeat("(", maxParseDepth+10) + "1" + strings.Repeat(")", maxParseDepth+10) + "; }\n"
	f := Parse([]byte(src))
	d := findDiagnostic(t, f.Diagnostics, DiagnosticMaximumDepth)
	if d.Recovery.Confidence != RecoverySuggested || d.Recovery.Kind != RecoveryNone {
		t.Fatalf("maximum-depth recovery must not be directly applicable: %+v", d)
	}
}

func TestKeywordQualifiedCallIsValid(t *testing.T) {
	t.Parallel()
	src := "main() { return callcmd::goto(playerid, params); }\n"
	f := Parse([]byte(src))
	if f.Broken || f.Root.HasError || f.HasParseErrors() || len(f.Diagnostics) != 0 {
		t.Fatalf("valid keyword-qualified call produced errors: %+v", f.Diagnostics)
	}
	call := f.Root.Children[0].Field("body").Children[0].Field("value")
	callee := call.Field("function")
	member := callee.Field("right")
	if callee.Kind != KindBinaryExpression || callee.Tok.Kind != token.ColonColon ||
		member == nil || member.Kind != KindIdentifier || member.Tok.Kind != token.KwGoto {
		t.Fatalf("keyword-qualified call has wrong CST shape: %+v", callee)
	}
}

func TestMalformedQualifiedNameEmitsDiagnostic(t *testing.T) {
	t.Parallel()
	src := "main() { broken::; }\n"
	f := Parse([]byte(src))
	if f.Broken || !f.Root.HasError || !f.HasParseErrors() {
		t.Fatalf("recoverable syntax error flags disagree: broken=%v root=%v diagnostics=%+v", f.Broken, f.Root.HasError, f.Diagnostics)
	}
	d := findDiagnostic(t, f.Diagnostics, DiagnosticMissingIdentifier)
	want := strings.Index(src, ";")
	if d.Range != (ByteRange{Start: want, End: want}) || d.Found.Kind != token.Semicolon {
		t.Fatalf("malformed qualified name has wrong anchor: %+v", d)
	}
	if !slices.Equal(d.Expected, []token.Kind{token.Identifier}) ||
		d.Recovery.Kind != RecoveryNone || d.Recovery.Confidence != RecoverySuggested {
		t.Fatalf("malformed qualified name has wrong expectation/recovery: %+v", d)
	}
	assertErrorFrontiersDiagnosed(t, f.Root, f.Diagnostics)
}

func findDiagnostic(t *testing.T, diagnostics []Diagnostic, code DiagnosticCode) Diagnostic {
	t.Helper()
	if d := findDiagnosticOptional(diagnostics, code); d != nil {
		return *d
	}
	t.Fatalf("diagnostic %q not found in %+v", code, diagnostics)
	return Diagnostic{}
}

func findDiagnosticOptional(diagnostics []Diagnostic, code DiagnosticCode) *Diagnostic {
	for i := range diagnostics {
		if diagnostics[i].Code == code {
			return &diagnostics[i]
		}
	}
	return nil
}

func applyRecovery(t *testing.T, source string, recovery Recovery) string {
	t.Helper()
	if recovery.Confidence != RecoveryExact || recovery.Range.Start < 0 || recovery.Range.End < recovery.Range.Start || recovery.Range.End > len(source) {
		t.Fatalf("invalid exact recovery: %+v", recovery)
	}
	return source[:recovery.Range.Start] + recovery.Replacement + source[recovery.Range.End:]
}

func shiftToken(tok *token.Token, offset int) {
	tok.Start.Offset += offset
	tok.End.Offset += offset
	for i := range tok.LeadingTrivia {
		tok.LeadingTrivia[i].Start.Offset += offset
		tok.LeadingTrivia[i].End.Offset += offset
	}
	for i := range tok.TrailingTrivia {
		tok.TrailingTrivia[i].Start.Offset += offset
		tok.TrailingTrivia[i].End.Offset += offset
	}
}

func assertErrorFrontiersDiagnosed(t *testing.T, node *Node, diagnostics []Diagnostic) bool {
	t.Helper()
	if node == nil || !node.HasError {
		return false
	}
	childHasError := false
	for _, child := range node.Children {
		if assertErrorFrontiersDiagnosed(t, child, diagnostics) {
			childHasError = true
		}
	}
	if !childHasError {
		covered := false
		for _, diagnostic := range diagnostics {
			covered = covered || diagnostic.Range.Start >= node.Start && diagnostic.Range.Start <= node.End
		}
		if !covered {
			t.Fatalf("error frontier has no structured diagnostic: %+v", node)
		}
	}
	return true
}
