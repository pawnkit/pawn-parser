package preprocess_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pawnkit/pawn-parser/preprocess"
)

func TestListingMatchesPawnCC31010ForSimpleSource(t *testing.T) {
	t.Parallel()

	result := preprocess.Run([]byte("main()\n    return 0\n"), preprocess.Options{
		URI:     "main.pwn", //nolint:goconst // The URI is part of the listing fixture.
		Listing: &preprocess.ListingOptions{},
	})
	want := []byte("#pragma ctrlchar 0x5c\n#pragma pack false\n#pragma semicolon false\n#pragma tabsize 8\n\n#file \"main.pwn\"\n#line 1\nmain()\n    return 0\n\n")
	if !bytes.Equal(result.Listing, want) {
		t.Fatalf("listing mismatch\nwant:\n%s\ngot:\n%s", want, result.Listing)
	}
}

func TestListingPreservesSourceLinesAndExpandsMacros(t *testing.T) {
	t.Parallel()

	source := []byte("#define VALUE 7\nmain()\n    return VALUE // expanded\n")
	result := preprocess.Run(source, preprocess.Options{
		URI: "main.pwn",
		Listing: &preprocess.ListingOptions{
			Semicolons: true,
		},
	})

	listing := string(result.Listing)
	for _, expected := range []string{
		"#pragma ctrlchar 0x5c\n",
		"#pragma pack false\n",
		"#pragma semicolon true\n",
		"#pragma tabsize 8\n",
		"#file \"main.pwn\"\n",
		"#line 2\n",
		"main()\n",
		"    return 7",
	} {
		if !strings.Contains(listing, expected) {
			t.Fatalf("listing does not contain %q:\n%s", expected, listing)
		}
	}
	if strings.Contains(listing, "#define") || strings.Contains(listing, "VALUE") || strings.Contains(listing, "expanded") {
		t.Fatalf("listing retained preprocessor input:\n%s", listing)
	}
}

func TestListingPreservesWhitespaceAcrossMacroCaptureOrigins(t *testing.T) {
	t.Parallel()

	result := preprocess.Run([]byte("#define WRAP(%0) return %0\nmain() { WRAP(0); }\n"), preprocess.Options{
		URI:     "main.pwn",
		Listing: &preprocess.ListingOptions{},
	})
	if listing := string(result.Listing); !strings.Contains(listing, "main() { return 0; }") {
		t.Fatalf("listing joined tokens separated in the macro body:\n%s", listing)
	}
}

func TestListingPreservesAdjacentBodyAndCaptureWhitespace(t *testing.T) {
	t.Parallel()

	result := preprocess.Run([]byte("#define WRITE(%0) op %0\nWRITE( value)\n"), preprocess.Options{
		URI:     "main.pwn",
		Listing: &preprocess.ListingOptions{},
	})
	if listing := string(result.Listing); !strings.Contains(listing, "op  value") {
		t.Fatalf("listing collapsed macro body and capture whitespace:\n%q", listing)
	}
}

func TestListingMarksMacroLineContinuationsLikePawnCC(t *testing.T) {
	t.Parallel()

	source := []byte("#define CHAIN:%0; forward %0; \\\n\tpublic %0;\nCHAIN:Callback;\n")
	result := preprocess.Run(source, preprocess.Options{
		URI:     "main.pwn",
		Listing: &preprocess.ListingOptions{},
	})
	if listing := string(result.Listing); !strings.Contains(listing, "forward Callback; \apublic Callback;") {
		t.Fatalf("listing did not retain PawnCC's continuation marker:\n%q", listing)
	}
}

func TestListingMarksIncludedFiles(t *testing.T) {
	t.Parallel()

	result := preprocess.Run([]byte("#include <helper>\nmain() { return Helper(); }\n"), preprocess.Options{
		URI: "main.pwn",
		Resolver: preprocess.MapResolver{
			"helper": []byte("stock Helper() { return 1; }\n"),
		},
		Listing: &preprocess.ListingOptions{},
	})

	listing := string(result.Listing)
	include := strings.Index(listing, "#file \"helper\"")
	root := strings.LastIndex(listing, "#file \"main.pwn\"")
	if include < 0 || root <= include {
		t.Fatalf("listing file order is wrong:\n%s", listing)
	}
}

func TestListingReturnsFromNonTerminatedIncludeWithoutBlankLine(t *testing.T) {
	t.Parallel()

	listing := string(preprocess.Run([]byte("#include <helper>\nnew root;\n"), preprocess.Options{
		URI: "main.pwn",
		Resolver: preprocess.MapResolver{
			"helper": []byte("new child;"),
		},
		Listing: &preprocess.ListingOptions{},
	}).Listing)
	if !strings.Contains(listing, "new child;\n#file \"main.pwn\"") {
		t.Fatalf("listing inserted a blank after a non-terminated include: %q", listing)
	}
}

func TestCompilerSymbolsAffectConditionsWithoutTextualExpansion(t *testing.T) {
	t.Parallel()

	result := preprocess.Run([]byte("#if cellbits == 32\nnew limit = cellmax;\n#endif\n"), preprocess.Options{
		URI: "main.pwn",
		Symbols: map[string]string{
			"cellbits": "32",
			"cellmax":  "2147483647",
		},
		Listing: &preprocess.ListingOptions{},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	listing := string(result.Listing)
	if !strings.Contains(listing, "new limit = cellmax;") {
		t.Fatalf("compiler symbol was textually expanded:\n%s", listing)
	}
}

func TestMacroNamesUsePawnCC31010SymbolLimit(t *testing.T) {
	t.Parallel()

	result := preprocess.Run([]byte("#define __OPEN_MP_LONG_SYMBOL_NAME_TEST\n#if defined __OPEN_MP_LONG_SYMBOL_NAME_TEST@\nnew matched = 1;\n#endif\n"), preprocess.Options{
		URI:     "main.pwn",
		Listing: &preprocess.ListingOptions{},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	if !strings.Contains(string(result.Listing), "new matched = 1;") {
		t.Fatalf("31-byte macro name was not matched:\n%s", result.Listing)
	}
}

func TestLiteralPatternMacroOnlyExpandsWhenSuffixMatches(t *testing.T) {
	t.Parallel()

	result := preprocess.Run([]byte("#define string:\nnew string[4];\nnew string:value;\n"), preprocess.Options{
		URI:     "main.pwn",
		Listing: &preprocess.ListingOptions{},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	listing := string(result.Listing)
	if !strings.Contains(listing, "new string[4];") {
		t.Fatalf("pattern macro expanded a prefix-only match:\n%s", listing)
	}
	if strings.Contains(listing, "string:value") {
		t.Fatalf("pattern macro did not expand an exact match:\n%s", listing)
	}
}

func TestCompatibilitySkipsRepeatedLogicalIncludes(t *testing.T) {
	t.Parallel()

	result := preprocess.Run([]byte("#include <dir\\helper>\n#include <other\\helper>\n"), preprocess.Options{
		URI:           "main.pwn",
		Compatibility: true,
		Resolver: preprocess.MapResolver{
			"dir\\helper":   []byte("new first;\n"),
			"other\\helper": []byte("new second;\n"),
		},
		Listing: &preprocess.ListingOptions{},
	})
	listing := string(result.Listing)
	if !strings.Contains(listing, "new first;") || strings.Contains(listing, "new second;") {
		t.Fatalf("automatic compatibility include guard failed:\n%s", listing)
	}
}

func TestCompatibilityKeepsForwardSlashIncludePathsDistinct(t *testing.T) {
	t.Parallel()

	result := preprocess.Run([]byte("#include <server/language.pwn>\n#include <player/language.pwn>\n"), preprocess.Options{
		URI:           "main.pwn",
		Compatibility: true,
		Resolver: preprocess.MapResolver{
			"server/language.pwn": []byte("new server_language;\n"),
			"player/language.pwn": []byte("new player_language;\n"),
		},
		Listing: &preprocess.ListingOptions{},
	})
	listing := string(result.Listing)
	if !strings.Contains(listing, "new server_language;") || !strings.Contains(listing, "new player_language;") {
		t.Fatalf("forward-slash include paths were collapsed by basename:\n%s", listing)
	}
}

func TestCompatibilityIncludeGuardCanBeUndefined(t *testing.T) {
	t.Parallel()

	result := preprocess.Run([]byte("#include <helper>\n#undef _inc_helper\n#include <helper>\n"), preprocess.Options{
		URI:           "main.pwn",
		Compatibility: true,
		Resolver: preprocess.MapResolver{
			"helper": []byte("new child;\n"),
		},
		Listing: &preprocess.ListingOptions{},
	})
	if diagnostics := result.Diagnostics; len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
	if count := strings.Count(string(result.Listing), "new child;"); count != 2 {
		t.Fatalf("undefined compatibility guard did not permit re-inclusion: count = %d\n%s", count, result.Listing)
	}
}

func TestListingRetainsActiveEndinputBeforeReturningToParent(t *testing.T) {
	t.Parallel()

	result := preprocess.Run([]byte("#include <guarded>\nnew root;\n"), preprocess.Options{
		URI: "main.pwn",
		Resolver: preprocess.MapResolver{
			"guarded": []byte("new child;\n#endinput\nnew hidden;\n"),
		},
		Listing: &preprocess.ListingOptions{},
	})
	listing := string(result.Listing)
	endinput := strings.Index(listing, "#endinput")
	parent := strings.LastIndex(listing, "#file \"main.pwn\"")
	if endinput < 0 || parent <= endinput {
		t.Fatalf("listing did not retain #endinput before returning to the parent:\n%s", listing)
	}
	if strings.Contains(listing, "hidden") {
		t.Fatalf("listing retained content after #endinput:\n%s", listing)
	}
}

func TestListingJoinsPlainSourceLineContinuations(t *testing.T) {
	t.Parallel()

	source := "new value = one + \\\n\t\ttwo;\nnew end;\n"
	result := preprocess.Run([]byte(source), preprocess.Options{
		Listing: &preprocess.ListingOptions{},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	listing := string(result.Listing)
	if !strings.Contains(listing, "new value = one + \a"+"two;") {
		t.Fatalf("plain continuation was not joined with PawnCC's marker: %q", listing)
	}
	if strings.Contains(listing, "\\\n") || !strings.Contains(listing, "new end;") {
		t.Fatalf("listing retained the physical continuation boundary: %q", listing)
	}
}

func TestListingJoinsLineContinuationsInsideStrings(t *testing.T) {
	t.Parallel()

	source := "new value[] = \"one\\\n\t\ttwo\\\n\t\tthree\";\nnew end;\n"
	result := preprocess.Run([]byte(source), preprocess.Options{
		Listing: &preprocess.ListingOptions{},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	if listing := string(result.Listing); !strings.Contains(listing, "#line 3\nnew value[] = \"one\a"+"two\a"+"three\";\nnew end;") || strings.Contains(listing, "\\\n") {
		t.Fatalf("string continuation was not normalized as one logical line: %q", listing)
	}
}

func TestListingUsesLastPhysicalLineForTerminatedStringContinuation(t *testing.T) {
	t.Parallel()

	source := "main()\n    print \"first \\\n            second\"\n    return 1\n"
	listing := string(preprocess.Run([]byte(source), preprocess.Options{
		URI:     "main.pwn",
		Listing: &preprocess.ListingOptions{},
	}).Listing)
	if !strings.Contains(listing, "#line 3\n    print \"first \a"+"second\"\n    return 1") {
		t.Fatalf("listing attributed a completed continuation to the following line: %q", listing)
	}
}

func TestListingDoesNotTerminateNonTerminatedRootSource(t *testing.T) {
	t.Parallel()

	listing := preprocess.Run([]byte("new value;"), preprocess.Options{
		URI:     "main.pwn",
		Listing: &preprocess.ListingOptions{},
	}).Listing
	if len(listing) == 0 || listing[len(listing)-1] == '\n' {
		t.Fatalf("listing added a terminal newline: %q", listing)
	}
}

func TestListingPreservesBackslashesInFilePaths(t *testing.T) {
	t.Parallel()

	listing := string(preprocess.Run([]byte("#include <entry>\n"), preprocess.Options{
		URI: "main.pwn",
		Resolver: listingPathResolver{
			content: []byte("new value;\n"),
			path:    `/includes/..\library\entry.inc`,
		},
		Listing: &preprocess.ListingOptions{},
	}).Listing)
	if !strings.Contains(listing, `#file "/includes/..\library\entry.inc"`) {
		t.Fatalf("listing escaped lexical backslashes: %q", listing)
	}
}

func TestListingPreservesSourceCRLF(t *testing.T) {
	t.Parallel()

	listing := preprocess.Run([]byte("new first;\r\nnew second;\r\n"), preprocess.Options{
		URI:     "main.pwn",
		Listing: &preprocess.ListingOptions{},
	}).Listing
	if !bytes.Contains(listing, []byte("new first;\r\nnew second;\r\n")) {
		t.Fatalf("listing normalized source line endings: %q", listing)
	}
}

func TestListingUsesLFForBlankCRLFSourceLines(t *testing.T) {
	t.Parallel()

	listing := preprocess.Run([]byte("new first;\r\n\r\nnew second;\r\n"), preprocess.Options{
		URI:     "main.pwn",
		Listing: &preprocess.ListingOptions{},
	}).Listing
	if !bytes.Contains(listing, []byte("new first;\r\n\nnew second;\r\n")) {
		t.Fatalf("listing retained CRLF for an emitted blank line: %q", listing)
	}
}

func TestListingPreservesIndentationAroundEmptyMacroExpansion(t *testing.T) {
	t.Parallel()

	listing := string(preprocess.Run([]byte("#define ERASE(%0)\nstock Test()\n{\n\tERASE(value)\n\treturn 1;\n}\n"), preprocess.Options{
		URI:     "main.pwn",
		Listing: &preprocess.ListingOptions{},
	}).Listing)
	if !strings.Contains(listing, "{\n\t\n\treturn 1;") {
		t.Fatalf("listing discarded indentation around an empty expansion: %q", listing)
	}
}

func TestListingSkipsContinuedBlockCommentOpeningLine(t *testing.T) {
	t.Parallel()

	source := "new before;\n/** section **\\\n<summary>text</summary>\n*/\nnew after;\n"
	listing := string(preprocess.Run([]byte(source), preprocess.Options{
		URI:     "main.pwn",
		Listing: &preprocess.ListingOptions{},
	}).Listing)
	if !strings.Contains(listing, "new before;\n#line 3\n\n\nnew after;") {
		t.Fatalf("listing retained a continued block-comment opening line: %q", listing)
	}
}

func TestListingCollapsesMultilineTrailingCommentToOneBlankLine(t *testing.T) {
	t.Parallel()

	source := "    assert Range >= Number      /* cannot select values\n                                 * without duplicates in the\n                                 * requested range */\n    new Index = 0\n"
	listing := string(preprocess.Run([]byte(source), preprocess.Options{
		URI:     "main.pwn",
		Listing: &preprocess.ListingOptions{},
	}).Listing)
	if !strings.Contains(listing, "assert Range >= Number") {
		t.Fatalf("listing discarded the code before the block comment: %q", listing)
	}
	if strings.Contains(listing, "requested range */") {
		t.Fatalf("listing retained the block comment: %q", listing)
	}
	start := strings.Index(listing, "Number")
	end := strings.Index(listing[start:], "new Index")
	if start < 0 || end < 0 || strings.Count(listing[start:start+end], "\n") != 2 {
		t.Fatalf("listing did not collapse the trailing block comment: %q", listing)
	}
}

type listingPathResolver struct {
	content []byte
	path    string
}

func (r listingPathResolver) Resolve(_, _ string, _ bool) ([]byte, string, bool) {
	return r.content, "canonical.inc", true
}

func (r listingPathResolver) ListingPath(_, _, _ string, _ bool, _ string) string {
	return r.path
}
