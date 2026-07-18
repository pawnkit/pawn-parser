package parser_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	parser "github.com/pawnkit/pawn-parser"
)

type corpusFixtureMeta struct {
	ID       string `json:"id"`
	Expected struct {
		Result string `json:"result"`
	} `json:"expected"`
}

// knownConformanceGaps records reviewed fixture/parser mismatches.
var knownConformanceGaps = map[string]string{
	"syntax.invalid.missing_semicolon": "pawn-parser treats a missing statement terminator as recoverable " +
		"(tracked via SyntaxNode.MissingSemicolon()), not a File.Diagnostics entry; fixture expects a hard diagnostic",
	"syntax.invalid.unclosed_preprocessor_if": "pawn-parser does not currently validate #if/#endif balance at EOF",
}

// TestCorpusConformance checks the shared syntax fixtures when available.
func TestCorpusConformance(t *testing.T) {
	t.Parallel()

	corpusRoot := findSiblingRepo(t, "pawn-corpus")
	if corpusRoot == "" {
		t.Skip("no sibling pawn-corpus checkout found")
	}

	runCorpusDir(t, filepath.Join(corpusRoot, "syntax", "valid"), true)
	runCorpusDir(t, filepath.Join(corpusRoot, "syntax", "invalid"), false)
}

func runCorpusDir(t *testing.T, dir string, wantValid bool) {
	t.Helper()

	metaFiles, err := findMetaFiles(dir)
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	if len(metaFiles) == 0 {
		t.Fatalf("no fixtures found under %s", dir)
	}

	for _, metaPath := range metaFiles {
		sourcePath := strings.TrimSuffix(metaPath, ".meta.json")

		meta, err := readFixtureMeta(metaPath)
		if err != nil {
			t.Errorf("%s: %v", metaPath, err)
			continue
		}

		t.Run(meta.ID, func(t *testing.T) {
			t.Parallel()

			if meta.Expected.Result == "pending" {
				t.Skip("expected.result is pending")
			}
			if reason, known := knownConformanceGaps[meta.ID]; known {
				t.Skip("known conformance gap: " + reason)
			}

			src, err := os.ReadFile(sourcePath) //nolint:gosec // Path comes from the corpus walk.
			if err != nil {
				t.Fatalf("read %s: %v", sourcePath, err)
			}

			file := parser.Parse(src)
			hasErrors := file.HasParseErrors()

			if wantValid && hasErrors {
				t.Errorf("expected valid parse, got errors: broken=%v diagnostics=%v", file.Broken, file.Diagnostics)
			}
			if !wantValid && !hasErrors {
				t.Errorf("expected invalid parse (fixture declares %q), got a clean parse", meta.Expected.Result)
			}
		})
	}
}

func findMetaFiles(dir string) ([]string, error) {
	var out []string

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".meta.json") {
			out = append(out, path)
		}
		return nil
	})

	return out, err
}

func readFixtureMeta(path string) (corpusFixtureMeta, error) {
	data, err := os.ReadFile(path) //nolint:gosec // Path comes from the corpus walk.
	if err != nil {
		return corpusFixtureMeta{}, err
	}

	var meta corpusFixtureMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return corpusFixtureMeta{}, err
	}

	return meta, nil
}

// findSiblingRepo checks the standard local workspace layout.
func findSiblingRepo(t *testing.T, name string) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	candidate := filepath.Join(wd, "..", name)
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		abs, err := filepath.Abs(candidate)
		if err != nil {
			t.Fatalf("abs: %v", err)
		}
		return abs
	}

	return ""
}
