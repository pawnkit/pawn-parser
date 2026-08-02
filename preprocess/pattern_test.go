package preprocess_test

import (
	"strings"
	"testing"

	"github.com/pawnkit/pawn-parser/preprocess"
)

func TestPawnTextualPatternsCaptureFixedSyntaxAndPasteTokens(t *testing.T) {
	t.Parallel()

	source := `#define JOIN:%0<> __%0
#define COMMAND:%0[%1](%2) public cmd_%0_%1(%2)
JOIN:value<>
COMMAND:open[admin](playerid, params[])
`
	result := preprocess.Run([]byte(source), preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	expanded := string(result.ExpandedSource)
	for _, expected := range []string{"__value", "cmd_open_admin"} {
		if !strings.Contains(expanded, expected) {
			t.Fatalf("textual token paste %q missing from %q", expected, expanded)
		}
	}
	for _, broken := range []string{"__ value", "cmd_ open", "open _ admin"} {
		if strings.Contains(expanded, broken) {
			t.Fatalf("textual token paste retained a token boundary %q in %q", broken, expanded)
		}
	}
}

func TestPawnTextualPatternsSkipNestedGroupsAndStrings(t *testing.T) {
	t.Parallel()

	source := `#define CALL(%0,%1) invoke(%0,%1)
CALL(one(1, 2), "a,b")
`
	result := preprocess.Run([]byte(source), preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	expanded := string(result.ExpandedSource)
	if !strings.Contains(expanded, `invoke ( one ( 1 , 2 ) , "a,b" )`) {
		t.Fatalf("nested delimiters were captured incorrectly: %q", expanded)
	}
}

func TestPawnTextualPatternsRescanSuffixChains(t *testing.T) {
	t.Parallel()

	source := `#define FIRST(%0)DROP$ SECOND(%0)DROP$
#define SECOND(%0)DROP$ done_%0
FIRST(item)DROP$
`
	result := preprocess.Run([]byte(source), preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	if expanded := string(result.ExpandedSource); !strings.Contains(expanded, "done_item") {
		t.Fatalf("replacement was not rescanned through its suffix: %q", expanded)
	}
}

func TestObjectMacroRescansUnmatchedTokenSuffix(t *testing.T) {
	t.Parallel()

	source := `#define _ADDR@ _ADDR@z
#define _ADDR@z$%0<> done_%0
#define MAKE _ADDR@$Callback<>
MAKE
`
	result := preprocess.Run([]byte(source), preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	if expanded := string(result.ExpandedSource); !strings.Contains(expanded, "done_Callback") || strings.Contains(expanded, "_ADDR@") {
		t.Fatalf("unmatched token suffix was not rescanned with the replacement: %q", expanded)
	}
}

func TestReplacementPastesAdjacentIdentifierCaptureBeforeRescan(t *testing.T) {
	t.Parallel()

	source := `#define _F<%0> %0
#define PREFIX%0\32; PREFIX
#define OUT(%0) _F<PREFIX>%0()
OUT(Name)
`
	result := preprocess.Run([]byte(source), preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	if expanded := string(result.ExpandedSource); !strings.Contains(expanded, "PREFIXName") || strings.Contains(expanded, "PREFIX Name") {
		t.Fatalf("adjacent replacement and capture were rescanned as separate identifiers: %q", expanded)
	}
}

func TestPatternConsumedWhitespaceIsNotRestoredAtReplacementBoundary(t *testing.T) {
	t.Parallel()

	source := `#define CALL%0(%1) _@%0(%1)
#define _@%0\32; _@
CALL Dialog_Set(playerid)
`
	result := preprocess.Run([]byte(source), preprocess.Options{
		Listing: &preprocess.ListingOptions{},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	listing := string(result.Listing)
	if !strings.Contains(listing, "_@Dialog_Set(playerid)") || strings.Contains(listing, "_@ Dialog_Set") {
		t.Fatalf("pattern-consumed whitespace was restored after substitution: %q", listing)
	}
}

func TestObjectPatternReplacesQualifiedPrefixOnce(t *testing.T) {
	t.Parallel()

	result := preprocess.Run([]byte("#define DB:: DB_\nenum DB::e_SYNCHRONOUS_MODE {\nDB::SYNCHRONOUS_OFF,\nDB::SYNCHRONOUS_NORMAL,\nDB::SYNCHRONOUS_FULL\n};\n"), preprocess.Options{
		Listing: &preprocess.ListingOptions{},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	expanded := string(result.ExpandedSource)
	if !strings.Contains(expanded, "DB_SYNCHRONOUS_FULL") || strings.Contains(expanded, "SYNCHRONOUS_FULLSYNCHRONOUS_FULL") {
		t.Fatalf("qualified suffix was substituted more than once: %q", expanded)
	}
	if listing := string(result.Listing); strings.Contains(listing, "SYNCHRONOUS_FULLSYNCHRONOUS_FULL") {
		t.Fatalf("listing reconstructed an already-pasted source suffix: %q", listing)
	}
}

func TestMacroReplacementRescansAgainstFollowingInvocationTokens(t *testing.T) {
	t.Parallel()

	source := `#define TARGET(%0) done_%0
#define ALIAS TARGET
ALIAS(value)
`
	result := preprocess.Run([]byte(source), preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	expanded := string(result.ExpandedSource)
	if !strings.Contains(expanded, "done_value") {
		t.Fatalf("replacement did not form an invocation with following tokens: %q", expanded)
	}
	if strings.Contains(expanded, "TARGET") {
		t.Fatalf("replacement-boundary invocation was left unexpanded: %q", expanded)
	}
}

func TestMacroReplacementBoundaryPreservesInvocationTrivia(t *testing.T) {
	t.Parallel()

	source := `#define ALS_MRET_(%0) expanded(%0)
#define ALS_MAIN_RET_ ALS_MRET_
ALS_MAIN_RET_ (D0NT_USE)
`
	result := preprocess.Run([]byte(source), preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	if expanded := string(result.ExpandedSource); !strings.Contains(expanded, "expanded ( D0NT_USE )") {
		t.Fatalf("replacement-boundary invocation lost its spelling or trivia: %q", expanded)
	}
}

func TestPawnTextualPatternsReuseLatestRepeatedCapture(t *testing.T) {
	t.Parallel()

	result := preprocess.Run([]byte("#define PICK:%0:%0; %0\nPICK:first:second;\n"), preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	if expanded := string(result.ExpandedSource); !strings.Contains(expanded, "second") || strings.Contains(expanded, "first") {
		t.Fatalf("repeated capture did not use the latest spelling: %q", expanded)
	}
}

func TestPatternMacroCanConsumeRepeatedSentinels(t *testing.T) {
	t.Parallel()

	source := `#define FLAGS<%0> e_NONE| CAT_%0| END|
#define e_NONE|%0\32;%1| e_%1|e_NONE|
#define e_END|e_NONE| e_NONE
FLAGS<OffRoad | Car>
`
	result := preprocess.Run([]byte(source), preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	expanded := string(result.ExpandedSource)
	for _, expected := range []string{"e_CAT_OffRoad", "e_Car", "e_NONE"} {
		if !strings.Contains(expanded, expected) {
			t.Fatalf("repeated sentinel expansion is missing %q: %q", expected, expanded)
		}
	}
	if strings.Contains(expanded, "END") || strings.Contains(expanded, "| Car") {
		t.Fatalf("repeated sentinel expansion stalled: %q", expanded)
	}
}

func TestRecursivePatternStopsWhenConsumedSpellingRepeats(t *testing.T) {
	t.Parallel()

	pattern := "LOOP:%0;"
	invocation := "LOOP:" + "value;"
	result := preprocess.Run([]byte("#define "+pattern+" "+pattern+"\n"+invocation+"\n"), preprocess.Options{})
	if result.Truncated {
		t.Fatalf("unchanged recursive pattern reached an expansion budget: diagnostics = %+v", result.Diagnostics)
	}
	if expanded := string(result.ExpandedSource); !strings.Contains(expanded, "LOOP : value ;") {
		t.Fatalf("recursive pattern did not stabilize at its repeated spelling: %q", expanded)
	}
}

func TestEmptySentinelReplacementPreservesPrecedingWhitespace(t *testing.T) {
	t.Parallel()

	source := `#define EMPTY$
#define DECL(%0) Item@%0 EMPTY$EMPTY$
DECL(Name),
`
	result := preprocess.Run([]byte(source), preprocess.Options{
		Listing: &preprocess.ListingOptions{},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	if listing := string(result.Listing); !strings.Contains(listing, "Item@Name ,") {
		t.Fatalf("empty sentinel replacement discarded preceding whitespace: %q", listing)
	}
}

func TestEmptyReplacementPreservesWhitespaceOnBothSides(t *testing.T) {
	t.Parallel()

	source := `#define EMPTY
#define NEXT #V
new value = EMPTY NEXT;
`
	result := preprocess.Run([]byte(source), preprocess.Options{
		Listing: &preprocess.ListingOptions{},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	if listing := string(result.Listing); !strings.Contains(listing, "value =  #V") {
		t.Fatalf("empty replacement collapsed its surrounding spaces: %q", listing)
	}
}

func TestNestedEmptySentinelsPreserveYSISizeofSpacing(t *testing.T) {
	t.Parallel()

	source := `#define F@a$
#define F@b|||
#define F@s(%0) (_:sizeof F@b|||(%0 F@a$F@b|||))
#define F@g(%0,%8) %0 F@a$
#define F@h:%0) %0 F@a$)
#define F@j:%7$%5(%0) %5A(F@g(Iter_Single@%0,),F@h:Iterator@%0)
#define Iter_Clear_InternalA(%0,%1) Iter_Clear_InternalC(_:%1,F@s(%1),F@s(%0),%0)
F@j:$Iter_Clear_Internal(Vehicle)
`
	result := preprocess.Run([]byte(source), preprocess.Options{
		Listing: &preprocess.ListingOptions{},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	if listing := string(result.Listing); !strings.Contains(listing, "Iter_Clear_InternalC(_:Iterator@Vehicle,(_:sizeof (Iterator@Vehicle  )),(_:sizeof (Iter_Single@Vehicle  )),Iter_Single@Vehicle  )") {
		t.Fatalf("nested empty sentinels collapsed YSI spacing: %q", listing)
	}
}

func TestPawnTextualPatternsDecodeEscapedDelimiter(t *testing.T) {
	t.Parallel()

	result := preprocess.Run([]byte("#define PREFIX%0\\32; replaced\nPREFIX value\n"), preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	if expanded := string(result.ExpandedSource); !strings.Contains(expanded, "replacedvalue") || strings.Contains(expanded, "replaced value") {
		t.Fatalf("escaped decimal delimiter was not decoded: %q", expanded)
	}
}

func TestPawnTextualPatternsMatchEscapedNewlineDelimiter(t *testing.T) {
	t.Parallel()

	result := preprocess.Run([]byte("#define EMIT%0\\32;%1\\10; done(%1);\nEMIT PROC // instruction comment\nEMIT NEXT\n"), preprocess.Options{
		Listing: &preprocess.ListingOptions{},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	if expanded := string(result.ExpandedSource); !strings.Contains(expanded, "done ( PROC )") || strings.Contains(expanded, "EMIT") || strings.Contains(expanded, "instruction") {
		t.Fatalf("escaped newline delimiter did not match the logical line: %q", expanded)
	}
	if listing := string(result.Listing); !strings.Contains(listing, "done(PROC );done(NEXT);") {
		t.Fatalf("listing did not join replacements that consumed source newlines: %q", listing)
	}
}

func TestPawnTextualPatternsHonorSemicolonMode(t *testing.T) {
	t.Parallel()

	source := []byte("#define RESULT(%0); value(%0)\nRESULT(item)\n")
	optional := preprocess.Run(source, preprocess.Options{Semicolons: false})
	if !strings.Contains(string(optional.ExpandedSource), "value") {
		t.Fatalf("optional semicolon pattern did not match: %q", optional.ExpandedSource)
	}
	required := preprocess.Run(source, preprocess.Options{Semicolons: true})
	if strings.Contains(string(required.ExpandedSource), "value") || !strings.Contains(string(required.ExpandedSource), "RESULT") {
		t.Fatalf("required semicolon pattern unexpectedly matched: %q", required.ExpandedSource)
	}
}

func TestChainedPatternMacroConsumesRequiredSemicolon(t *testing.T) {
	t.Parallel()

	source := `#define __pragma("%0"%1) __pragma_%0(%1)
#define __pragma_naked(); {return 0;}
__pragma("naked");
`
	result := preprocess.Run([]byte(source), preprocess.Options{
		Semicolons: true,
		Listing:    &preprocess.ListingOptions{Semicolons: true},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	expanded := string(result.ExpandedSource)
	if !strings.Contains(expanded, "{ return 0 ; }") || strings.Contains(expanded, "} ;") {
		t.Fatalf("chained pattern left its required semicolon behind: %q; macros = %#v", expanded, result.Macros)
	}
	if listing := string(result.Listing); strings.Contains(listing, "};") || !strings.Contains(listing, "{return 0;}") {
		t.Fatalf("listing restored a semicolon consumed by the chained pattern: %q", listing)
	}
}

func TestPatternMacroMatchesAcrossContinuedSourceLines(t *testing.T) {
	t.Parallel()

	source := "#define WRAP(%0); done(%0)\nWRAP(one \\\n\ttwo);\n"
	result := preprocess.Run([]byte(source), preprocess.Options{
		Semicolons: true,
		Listing:    &preprocess.ListingOptions{Semicolons: true},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	if expanded := string(result.ExpandedSource); strings.Contains(expanded, "WRAP") || !strings.Contains(expanded, "done") {
		t.Fatalf("continued invocation was not expanded: %q", expanded)
	}
	if listing := string(result.Listing); !strings.Contains(listing, "done(one \atwo)") {
		t.Fatalf("continued invocation listing differs from PawnCC: %q", listing)
	}
}

func TestPawnTextualPatternsMatchKeywordPrefixes(t *testing.T) {
	t.Parallel()

	source := `#define native%9void:%0(%1); %0(%1);native __%0() = %0;
native void:print(const string[]);
`
	result := preprocess.Run([]byte(source), preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	expanded := string(result.ExpandedSource)
	for _, expected := range []string{"__print", "print"} {
		if !strings.Contains(expanded, expected) {
			t.Fatalf("keyword-prefix pattern result %q missing from %q", expected, expanded)
		}
	}
}

func TestPawnTextualPatternsDoNotSubstituteInsideStrings(t *testing.T) {
	t.Parallel()

	result := preprocess.Run([]byte("#define SHOW(%0) print(\"%0\", %0)\nSHOW(value)\n"), preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	expanded := string(result.ExpandedSource)
	if !strings.Contains(expanded, `"%0"`) || !strings.Contains(expanded, "value") {
		t.Fatalf("string substitution behavior differs from PawnCC: %q", expanded)
	}
}
