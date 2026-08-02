package preprocess_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pawnkit/pawn-parser/preprocess"
)

type delayedCancelContext struct {
	checks atomic.Int32
	after  int32
}

func TestForwardingMacroExposesReplacementCallable(t *testing.T) {
	t.Parallel()
	result := preprocess.Run([]byte("#define PlayerDialog_Show(%0,%1, \\\n Dialog_Open(%0,#%1,\n"), preprocess.Options{})
	macro, ok := result.Macros["PlayerDialog_Show"]
	if !ok {
		t.Fatal("macro missing")
	}
	if got, ok := macro.ReplacementCallable(); !ok || got != "Dialog_Open" {
		t.Fatalf("replacement callable = %q, %v; macro = %#v", got, ok, macro)
	}
}

func (c *delayedCancelContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *delayedCancelContext) Done() <-chan struct{}       { return nil }
func (c *delayedCancelContext) Value(any) any               { return nil }
func (c *delayedCancelContext) Err() error {
	if c.checks.Add(1) > c.after {
		return context.Canceled
	}
	return nil
}

func expandedText(t *testing.T, r *preprocess.Result) string {
	t.Helper()
	var sb strings.Builder
	for _, tok := range r.ExpandedTokens {
		if tok.Kind.String() == "EOF" {
			continue
		}
		sb.WriteString(tok.Text(r.ExpandedSource))
		sb.WriteByte(' ')
	}
	return sb.String()
}

func TestRunContextStopsDuringPreprocessing(t *testing.T) {
	t.Parallel()
	src := []byte("#define DUP(%0) %0 %0\n" + strings.Repeat("DUP(value)\n", 2_000))
	ctx := &delayedCancelContext{after: 1}

	result, err := preprocess.RunContext(ctx, src, preprocess.Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if result != nil {
		t.Fatal("cancelled preprocessing returned a result")
	}
}

func TestObjectMacroExpansion(t *testing.T) {
	t.Parallel()
	src := "#define MAX_ZONES 32\nnew zones = MAX_ZONES;\n"
	r := preprocess.Run([]byte(src), preprocess.Options{})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", r.Diagnostics)
	}
	got := expandedText(t, r)
	if !strings.Contains(got, "zones = 32") {
		t.Fatalf("expected object macro expansion, got %q", got)
	}
}

func TestImplicitPrefixIsProcessedBeforeRootSource(t *testing.T) {
	t.Parallel()

	result := preprocess.Run([]byte("new root = PREFIX_VALUE;\n"), preprocess.Options{
		URI:    "main.pwn", //nolint:goconst // The URI is part of the source fixture.
		Prefix: "default.inc",
		Resolver: preprocess.MapResolver{
			"default.inc": []byte("#define PREFIX_VALUE 7\nnew prefix;\n"),
		},
		Listing: &preprocess.ListingOptions{},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	if expanded := string(result.ExpandedSource); !strings.Contains(expanded, "prefix") || !strings.Contains(expanded, "root = 7") {
		t.Fatalf("prefix was not processed before the root: %q", expanded)
	}
	listing := string(result.Listing)
	rootFirst := strings.Index(listing, "#file \"main.pwn\"")
	prefix := strings.Index(listing, "#file \"default.inc\"")
	rootReturn := strings.LastIndex(listing, "#file \"main.pwn\"")
	if rootFirst < 0 || prefix <= rootFirst || rootReturn <= prefix {
		t.Fatalf("prefix listing file order is wrong: %q", listing)
	}
}

func TestMissingRequiredImplicitPrefixIsDiagnosed(t *testing.T) {
	t.Parallel()

	result := preprocess.Run([]byte("new root;\n"), preprocess.Options{
		URI:      "main.pwn",
		Prefix:   "required.inc",
		Resolver: preprocess.MapResolver{},
	})
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != preprocess.CodeIncludeNotFound {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
}

func TestMissingOptionalImplicitPrefixIsSilent(t *testing.T) {
	t.Parallel()

	result := preprocess.Run([]byte("new root;\n"), preprocess.Options{
		URI:            "main.pwn",
		Prefix:         "default.inc",
		PrefixOptional: true,
		Resolver:       preprocess.MapResolver{},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
}

func TestFunctionMacroExpansion(t *testing.T) {
	t.Parallel()
	src := "#define SQR(%0) ((%0) * (%0))\nnew x = SQR(zones + 1);\n"
	r := preprocess.Run([]byte(src), preprocess.Options{})
	got := expandedText(t, r)
	want := "( ( zones + 1 ) * ( zones + 1 ) )"
	if !strings.Contains(got, want) {
		t.Fatalf("expected %q in %q", want, got)
	}
}

func TestMacroInvocationsRetainSourceRanges(t *testing.T) {
	t.Parallel()
	src := "#define VALUE 1\n#define SQR(%0) ((%0) * (%0))\nnew x = VALUE + SQR(2);\n"
	result := preprocess.Run([]byte(src), preprocess.Options{})
	if len(result.MacroInvocations) != 2 {
		t.Fatalf("invocations = %#v", result.MacroInvocations)
	}
	for index, spelling := range []string{"VALUE", "SQR(2)"} {
		start := strings.LastIndex(src, spelling)
		got := result.MacroInvocations[index]
		if got.File != 0 || got.Range.Start != start || got.Range.End != start+len(spelling) {
			t.Fatalf("invocation %d = %#v", index, got)
		}
	}
}

func TestFunctionMacroArgumentRepetition(t *testing.T) {
	t.Parallel()
	src := "#define TWICE(%0) %0 %0\nTWICE(zones++;)\n"
	r := preprocess.Run([]byte(src), preprocess.Options{})
	got := expandedText(t, r)
	if strings.Count(got, "zones") != 2 {
		t.Fatalf("expected argument to be repeated twice, got %q", got)
	}
}

func TestFunctionMacroFinalParameterConsumesCommas(t *testing.T) {
	t.Parallel()
	src := "#define COPY(%0,%1) (%0 = %1)\nCOPY(value, 1, 2)\n"
	r := preprocess.Run([]byte(src), preprocess.Options{})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", r.Diagnostics)
	}
	if got := string(r.ExpandedSource); !strings.Contains(got, "value = 1 , 2") {
		t.Fatalf("expanded source = %q", got)
	}
}

func TestFunctionMacroAcceptsEmptyArgument(t *testing.T) {
	t.Parallel()
	src := "#define EMPTY(%0) value\nEMPTY()\n"
	r := preprocess.Run([]byte(src), preprocess.Options{})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", r.Diagnostics)
	}
	if got := string(r.ExpandedSource); !strings.Contains(got, "value") {
		t.Fatalf("expanded source = %q", got)
	}
}

func TestEmitDirectiveIsAccepted(t *testing.T) {
	t.Parallel()
	r := preprocess.Run([]byte("#emit CONST.pri 1\n"), preprocess.Options{})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", r.Diagnostics)
	}
}

func TestPrefixMacroDoesNotReportArgumentMismatch(t *testing.T) {
	t.Parallel()
	src := "#define main( ALS_main_:PP_main(\nmain() {}\n"
	r := preprocess.Run([]byte(src), preprocess.Options{})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", r.Diagnostics)
	}
}

func TestMultiArgFunctionMacro(t *testing.T) {
	t.Parallel()
	src := "#define CLAMP(%0,%1,%2) ((%0) < (%1) ? (%1) : ((%0) > (%2) ? (%2) : (%0)))\n" +
		"new c = CLAMP(zones, 0, 16);\n"
	r := preprocess.Run([]byte(src), preprocess.Options{})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", r.Diagnostics)
	}
	got := expandedText(t, r)
	if !strings.Contains(got, "zones") || !strings.Contains(got, "16") {
		t.Fatalf("expected substitution, got %q", got)
	}
}

func TestMacroParameterLabelsUseDeclarationOrder(t *testing.T) {
	t.Parallel()
	src := "#define PICK(%1) (%1)\n" +
		"#define PAIR(%0,%2) ((%0) + (%2))\n" +
		"new value = PICK(3) + PAIR(4, 5);\n"
	r := preprocess.Run([]byte(src), preprocess.Options{})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", r.Diagnostics)
	}
	got := expandedText(t, r)
	if !strings.Contains(got, "( 3 )") || !strings.Contains(got, "( ( 4 ) + ( 5 ) )") {
		t.Fatalf("expected declaration-order substitution, got %q", got)
	}
}

func TestNamedParamMacro(t *testing.T) {
	t.Parallel()
	src := "#define ADD(a,b) ((a) + (b))\nnew x = ADD(1, 2);\n"
	r := preprocess.Run([]byte(src), preprocess.Options{})
	got := expandedText(t, r)
	if !strings.Contains(got, "( 1 ) + ( 2 )") {
		t.Fatalf("expected named-param substitution, got %q", got)
	}
}

func TestMacroSelfReferenceDoesNotRecurse(t *testing.T) {
	t.Parallel()
	foo := "FOO"
	src := "#define " + foo + " " + foo + " + 1\nnew x = " + foo + ";\n"
	r := preprocess.Run([]byte(src), preprocess.Options{})
	got := expandedText(t, r)
	if !strings.Contains(got, "FOO + 1") {
		t.Fatalf("expected one level of self-reference expansion, got %q", got)
	}
}

func TestUndef(t *testing.T) {
	t.Parallel()
	src := "#define FOO 1\n#undef FOO\nnew x = FOO;\n"
	r := preprocess.Run([]byte(src), preprocess.Options{})
	got := expandedText(t, r)
	if !strings.Contains(got, "x = FOO") {
		t.Fatalf("expected FOO to stay unexpanded after #undef, got %q", got)
	}
}

func TestConditionalCompilationDefined(t *testing.T) {
	t.Parallel()
	src := "#define FEATURE\n#if defined FEATURE\nnew a = 1;\n#else\nnew a = 2;\n#endif\n"
	r := preprocess.Run([]byte(src), preprocess.Options{})
	got := expandedText(t, r)
	if !strings.Contains(got, "a = 1") || strings.Contains(got, "a = 2") {
		t.Fatalf("expected the defined() branch to be active, got %q", got)
	}
	if len(r.Branches) != 2 {
		t.Fatalf("expected 2 branches, got %d: %+v", len(r.Branches), r.Branches)
	}
	if !r.Branches[0].Active || r.Branches[1].Active {
		t.Fatalf("branch activity mismatch: %+v", r.Branches)
	}
}

func TestConditionalExpandsFunctionMacroBeforeDefinedEvaluation(t *testing.T) {
	t.Parallel()

	source := `#define YSI_KEYWORD(%0) ((defined YSI_COMPATIBILITY_MODE && defined YSI_KEYWORD_%0) || (!defined YSI_COMPATIBILITY_MODE && !defined YSI_NO_KEYWORD_%0))
#if YSI_KEYWORD(if)
new keyword_enabled = 1;
#else
new keyword_enabled = 0;
#endif
`
	result := preprocess.Run([]byte(source), preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	expanded := string(result.ExpandedSource)
	if !strings.Contains(expanded, "keyword_enabled = 1") || strings.Contains(expanded, "keyword_enabled = 0") {
		t.Fatalf("function-like condition macro selected the wrong branch: %q", expanded)
	}
}

func TestConditionalUsesConstantWhenSamePrefixPatternDoesNotMatch(t *testing.T) {
	t.Parallel()

	source := `const REQUIRE_SEMICOLON = 1;
#define REQUIRE_SEMICOLON%0; 0%0
#if REQUIRE_SEMICOLON == 1
new semicolons_enabled = 1;
#else
new semicolons_enabled = 0;
#endif
`
	result := preprocess.Run([]byte(source), preprocess.Options{Semicolons: true})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	expanded := string(result.ExpandedSource)
	if !strings.Contains(expanded, "semicolons_enabled = 1") || strings.Contains(expanded, "semicolons_enabled = 0") {
		t.Fatalf("pattern macro shadowed the same-name constant: %q", expanded)
	}
}

func TestConditionsEvaluateInferredArraySizeof(t *testing.T) {
	t.Parallel()

	source := `static stock const unpacked[] = " ";
#if sizeof (unpacked) == 2
new unpacked_size = 2;
#else
new unpacked_size = 0;
#endif
`
	result := preprocess.Run([]byte(source), preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	expanded := string(result.ExpandedSource)
	if !strings.Contains(expanded, "unpacked_size = 2") || strings.Contains(expanded, "unpacked_size = 0") {
		t.Fatalf("inferred string array sizeof selected the wrong branch: %q", expanded)
	}
}

func TestConditionsEvaluateRawByteStringSizeof(t *testing.T) {
	t.Parallel()

	source := append([]byte("static stock const codepage[] = \"A"), 0xd2)
	source = append(source, []byte("\";\n#if sizeof (codepage) == 3\nnew codepage_size = 3;\n#else\nnew codepage_size = 0;\n#endif\n")...)
	result := preprocess.Run(source, preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	expanded := string(result.ExpandedSource)
	if !strings.Contains(expanded, "codepage_size = 3") || strings.Contains(expanded, "codepage_size = 0") {
		t.Fatalf("raw byte string sizeof selected the wrong branch: %q", expanded)
	}
}

func TestConditionsEvaluateEnumFieldSizeof(t *testing.T) {
	t.Parallel()

	source := `enum Layout
{
    LayoutEntry[3],
    LayoutAfter
}
static stock const values[Layout];
#if sizeof (values[LayoutEntry]) == 3
new enum_field_size = 3;
#else
new enum_field_size = 0;
#endif
`
	result := preprocess.Run([]byte(source), preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	expanded := string(result.ExpandedSource)
	if !strings.Contains(expanded, "enum_field_size = 3") || strings.Contains(expanded, "enum_field_size = 0") {
		t.Fatalf("enum field sizeof selected the wrong branch: %q", expanded)
	}
}

func TestDefinedObservesEarlierFunctionDeclarations(t *testing.T) {
	t.Parallel()
	src := `native __print(const value[]);
forward Future(value);
stock Implemented(value)
{
    return value;
}
#if defined __print && defined Future && defined Implemented
new selected = 1;
#else
new selected = 0;
#endif
`
	result := preprocess.Run([]byte(src), preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
	}
	expanded := expandedText(t, result)
	if !strings.Contains(expanded, "selected = 1") || strings.Contains(expanded, "selected = 0") {
		t.Fatalf("earlier function declarations were not visible to defined: %q", expanded)
	}
}

func TestConditionsReadEarlierConstDeclarations(t *testing.T) {
	t.Parallel()

	source := `const __debug = debug;
#if __debug > 0
new enabled;
#else
new disabled;
#endif
`
	result := preprocess.Run([]byte(source), preprocess.Options{
		Symbols: map[string]string{"debug": "2"},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	expanded := string(result.ExpandedSource)
	if !strings.Contains(expanded, "enabled") || strings.Contains(expanded, "disabled") {
		t.Fatalf("condition did not use the declared constant value: %q", expanded)
	}
}

func TestResolvedConstantsBecomeVisibleOnlyAfterTheirDeclaration(t *testing.T) {
	t.Parallel()

	source := `#if defined Layout
new leaked;
#endif
const Layout = __addressof(Second) - __addressof(First);
#if Layout == 60
new selected;
#endif
`
	result := preprocess.Run([]byte(source), preprocess.Options{
		ResolvedConstants: map[string]string{"Layout": "60"},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	expanded := string(result.ExpandedSource)
	if strings.Contains(expanded, "leaked") || !strings.Contains(expanded, "selected") {
		t.Fatalf("resolved constant did not preserve declaration visibility: %q", expanded)
	}
}

func TestDefinedObservesVariablesAndFunctionParametersInScope(t *testing.T) {
	t.Parallel()
	src := `new globalValue;
#if defined globalValue
new globalSelected;
#endif
main(playerid)
{
#if defined playerid
    new parameterSelected;
#endif
    new localValue;
#if defined localValue
    new localSelected;
#endif
}
`
	result := preprocess.Run([]byte(src), preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
	}
	expanded := expandedText(t, result)
	for _, name := range []string{"globalSelected", "parameterSelected", "localSelected"} {
		if !strings.Contains(expanded, name) {
			t.Fatalf("declaration %q was not visible to defined: %q", name, expanded)
		}
	}
}

func TestListingPassSeedsFunctionsDiscoveredLaterInFirstPass(t *testing.T) {
	t.Parallel()
	src := `#if defined LaterFunction
new functionVisible = 1;
#else
new functionVisible = 0;
#endif
#if defined laterVariable
new variableVisible = 1;
#else
new variableVisible = 0;
#endif
stock LaterFunction() { return 1; }
new laterVariable;
`
	result := preprocess.Run([]byte(src), preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
	}
	expanded := expandedText(t, result)
	if !strings.Contains(expanded, "functionVisible = 1") || strings.Contains(expanded, "functionVisible = 0") {
		t.Fatalf("first-pass function table was not seeded into the output pass: %q", expanded)
	}
	if !strings.Contains(expanded, "variableVisible = 0") || strings.Contains(expanded, "variableVisible = 1") {
		t.Fatalf("later variable incorrectly inherited a defined flag from the discovery pass: %q", expanded)
	}
}

func TestTaggedUseBeforeDeclarationTriggersAdditionalDiscoveryPass(t *testing.T) {
	t.Parallel()
	src := `#if defined SecondPassMarker
#define PASS 2
#elseif defined FirstPassMarker
#define PASS 1
#else
#define PASS 0
#endif
#if PASS == 0
stock FirstPassMarker() {}
#else
stock FirstPassMarkerFallback() {}
#endif
#if PASS == 1
stock SecondPassMarker() {}
#else
stock SecondPassMarkerFallback() {}
#endif
new passValue = PASS;
main()
{
    TaggedResult();
}
Float:TaggedResult() { return 1.0; }
`
	result := preprocess.Run([]byte(src), preprocess.Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
	}
	expanded := expandedText(t, result)
	if !strings.Contains(expanded, "passValue = 2") {
		t.Fatalf("tagged use-before-declaration did not trigger PawnCC's extra discovery pass: %q", expanded)
	}
}

func TestConditionalCompilationElseif(t *testing.T) {
	t.Parallel()
	src := "#define HUD_VERSION 2\n" +
		"#if HUD_VERSION >= 2\nnew v = 2;\n#elseif HUD_VERSION == 1\nnew v = 1;\n#else\nnew v = 0;\n#endif\n"
	r := preprocess.Run([]byte(src), preprocess.Options{})
	got := expandedText(t, r)
	if !strings.Contains(got, "v = 2") {
		t.Fatalf("expected first branch active, got %q", got)
	}
}

func TestConditionalNested(t *testing.T) {
	t.Parallel()
	src := "#if 1\n#if 0\nnew a = 1;\n#else\nnew a = 2;\n#endif\n#endif\n"
	r := preprocess.Run([]byte(src), preprocess.Options{})
	got := expandedText(t, r)
	if !strings.Contains(got, "a = 2") || strings.Contains(got, "a = 1") {
		t.Fatalf("expected nested else branch, got %q", got)
	}
}

func TestInactiveBranchDefinesDoNotLeak(t *testing.T) {
	t.Parallel()
	src := "#if 0\n#define SHOULD_NOT_EXIST 1\n#endif\nnew x = SHOULD_NOT_EXIST;\n"
	r := preprocess.Run([]byte(src), preprocess.Options{})
	got := expandedText(t, r)
	if !strings.Contains(got, "x = SHOULD_NOT_EXIST") {
		t.Fatalf("macro defined in inactive branch must not take effect, got %q", got)
	}
}

func TestIncludeGuardPattern(t *testing.T) {
	t.Parallel()
	resolver := preprocess.MapResolver{
		"helper.inc": []byte("#if defined _INC_HELPER\n#endinput\n#endif\n#define _INC_HELPER\nstock Helper() { return 1; }\n"),
	}
	src := "#include \"helper.inc\"\n#include \"helper.inc\"\nmain() { Helper(); }\n"
	r := preprocess.Run([]byte(src), preprocess.Options{Resolver: resolver})
	if len(r.Includes) != 2 {
		t.Fatalf("expected 2 include directives recorded, got %d", len(r.Includes))
	}
	got := expandedText(t, r)
	if strings.Count(got, "stock") != 1 {
		t.Fatalf("expected include guard to prevent double inclusion, got %q", got)
	}
}

func TestIncludeGuardBreaksRecursiveInclude(t *testing.T) {
	t.Parallel()
	resolver := preprocess.MapResolver{
		"a.inc": []byte("#if defined _INC_A\n#endinput\n#endif\n#define _INC_A\n#include \"b.inc\"\n"),
		"b.inc": []byte("#include \"a.inc\"\n"),
	}
	result := preprocess.Run([]byte("#include \"a.inc\"\n"), preprocess.Options{Resolver: resolver})
	for _, item := range result.Diagnostics {
		if item.Code == preprocess.CodeIncludeCycle {
			t.Fatalf("guarded include reported a cycle: %+v", item)
		}
	}
}

func TestUnguardedRecursiveIncludeReportsCycle(t *testing.T) {
	t.Parallel()
	resolver := preprocess.MapResolver{
		"a.inc": []byte("#include \"b.inc\"\n"),
		"b.inc": []byte("#include \"a.inc\"\n"),
	}
	result := preprocess.Run([]byte("#include \"a.inc\"\n"), preprocess.Options{Resolver: resolver})
	for _, item := range result.Diagnostics {
		if item.Code == preprocess.CodeIncludeCycle {
			return
		}
	}
	t.Fatalf("cycle diagnostic missing: %+v", result.Diagnostics)
}

func TestTokenCacheProducesIdenticalResultsAcrossRuns(t *testing.T) {
	t.Parallel()
	resolver := preprocess.MapResolver{
		"helper.inc": []byte("#define HELPER_VALUE 7\nstock Helper() { return HELPER_VALUE; }\n"),
	}
	src := "#include \"helper.inc\"\nmain() { Helper(); }\n"
	cache := preprocess.NewTokenCache()

	first := preprocess.Run([]byte(src), preprocess.Options{Resolver: resolver, TokenCache: cache})
	second := preprocess.Run([]byte(src), preprocess.Options{Resolver: resolver, TokenCache: cache})

	firstText := expandedText(t, first)
	secondText := expandedText(t, second)
	if firstText != secondText {
		t.Fatalf("expanded output changed across cached runs:\n%q\n%q", firstText, secondText)
	}
	if len(first.Diagnostics) != 0 || len(second.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v / %v", first.Diagnostics, second.Diagnostics)
	}
}

func TestTryIncludeMissingIsSilent(t *testing.T) {
	t.Parallel()
	src := "#tryinclude <does_not_exist>\nnew x = 1;\n"
	r := preprocess.Run([]byte(src), preprocess.Options{Resolver: preprocess.MapResolver{}})
	for _, d := range r.Diagnostics {
		if d.Code == preprocess.CodeIncludeNotFound {
			t.Fatalf("tryinclude of a missing file must not be an error: %+v", d)
		}
	}
}

func TestIncludeMissingIsError(t *testing.T) {
	t.Parallel()
	src := "#include <does_not_exist>\nnew x = 1;\n"
	r := preprocess.Run([]byte(src), preprocess.Options{Resolver: preprocess.MapResolver{}})
	found := false
	for _, d := range r.Diagnostics {
		if d.Code == preprocess.CodeIncludeNotFound {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected include-not-found diagnostic, got %+v", r.Diagnostics)
	}
}

func TestNoResolverLeavesIncludeUnresolved(t *testing.T) {
	t.Parallel()
	src := "#include <a_samp>\n"
	r := preprocess.Run([]byte(src), preprocess.Options{})
	if len(r.Includes) != 1 || r.Includes[0].Resolved {
		t.Fatalf("expected an unresolved include record, got %+v", r.Includes)
	}
	for _, d := range r.Diagnostics {
		if d.Code == preprocess.CodeIncludeNotFound {
			t.Fatalf("no resolver configured should not itself be an error: %+v", d)
		}
	}
}

func TestErrorAndWarningDirectives(t *testing.T) {
	t.Parallel()
	src := "#warning something is off\n#if 0\n#error should not fire\n#endif\n"
	r := preprocess.Run([]byte(src), preprocess.Options{})
	var warn, errs int
	for _, d := range r.Diagnostics {
		//nolint:exhaustive // This test checks only warning and error directives.
		switch d.Code {
		case preprocess.CodeUserWarning:
			warn++
		case preprocess.CodeUserError:
			errs++
		}
	}
	if warn != 1 || errs != 0 {
		t.Fatalf("expected 1 warning and 0 errors, got warn=%d errs=%d diags=%+v", warn, errs, r.Diagnostics)
	}
}

func TestAssertDirective(t *testing.T) {
	t.Parallel()
	src := "#assert 1 == 2\n"
	r := preprocess.Run([]byte(src), preprocess.Options{})
	found := false
	for _, d := range r.Diagnostics {
		if d.Code == preprocess.CodeAssertFailed {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an assert-failed diagnostic, got %+v", r.Diagnostics)
	}
}

func TestEndinputStopsProcessing(t *testing.T) {
	t.Parallel()
	src := "new a = 1;\n#endinput\nnew b = 2;\n"
	r := preprocess.Run([]byte(src), preprocess.Options{})
	got := expandedText(t, r)
	if !strings.Contains(got, "a = 1") || strings.Contains(got, "b = 2") {
		t.Fatalf("expected content after #endinput to be dropped, got %q", got)
	}
}

func TestPredefinedMacros(t *testing.T) {
	t.Parallel()
	src := "new a = OPEN_MP;\n"
	r := preprocess.Run([]byte(src), preprocess.Options{Predefined: map[string]string{"OPEN_MP": "1"}})
	got := expandedText(t, r)
	if !strings.Contains(got, "a = 1") {
		t.Fatalf("expected predefined macro expansion, got %q", got)
	}
}

func TestDeterministicOutput(t *testing.T) {
	t.Parallel()
	src := "#define SQR(%0) ((%0)*(%0))\nnew x = SQR(1+2);\n#if defined SQR\nnew y = 1;\n#endif\n"
	r1 := preprocess.Run([]byte(src), preprocess.Options{})
	r2 := preprocess.Run([]byte(src), preprocess.Options{})
	if expandedText(t, r1) != expandedText(t, r2) {
		t.Fatalf("expected deterministic expansion output")
	}
	if len(r1.Branches) != len(r2.Branches) || len(r1.Diagnostics) != len(r2.Diagnostics) {
		t.Fatalf("expected deterministic branch/diagnostic counts")
	}
}
