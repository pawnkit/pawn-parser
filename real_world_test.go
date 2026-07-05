package parser

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const realWorldFixtureDir = "testdata/real-world"

type realWorldFixture struct {
	project string
	path    string
}

func TestRealWorldFixtures(t *testing.T) { //nolint:paralleltest // Sequential parsing bounds peak corpus memory.
	fixtures := readRealWorldFixtures(t)
	if len(fixtures) != 45 {
		t.Fatalf("expected 45 real-world fixtures, got %d", len(fixtures))
	}

	for _, fixture := range fixtures { //nolint:paralleltest // Subtests intentionally run sequentially.
		name := fixture.project + "/" + fixture.path
		t.Run(name, func(t *testing.T) {
			sourcePath := filepath.Join(realWorldFixtureDir, fixture.project, filepath.FromSlash(fixture.path))
			// #nosec G304
			source, err := os.ReadFile(sourcePath)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if len(source) == 0 {
				t.Fatal("fixture is empty")
			}

			file := Parse(source)
			if file == nil || file.Root == nil {
				t.Fatal("Parse returned no CST")
			}
			if file.Broken {
				t.Fatal("parser marked the fixture as broken")
			}
			if file.Root.Kind != KindSourceFile {
				t.Fatalf("root kind = %s, want %s", file.Root.Kind, KindSourceFile)
			}
			if file.Root.Start != 0 || file.Root.End != len(source) {
				t.Fatalf("root span = [%d:%d], want [0:%d]", file.Root.Start, file.Root.End, len(source))
			}
			if got := file.Root.Text(source); got != string(source) {
				t.Fatal("root node does not preserve the complete source")
			}
			if len(file.Root.Children) == 0 {
				t.Fatal("fixture produced an empty CST")
			}
			assertCleanRealWorldCST(t, file.Root, source)
		})
	}
}

func assertCleanRealWorldCST(t *testing.T, root *Node, source []byte) {
	t.Helper()
	var problems []string
	var visit func(*Node, string)
	visit = func(node *Node, path string) {
		path += "/" + node.Kind.String()
		childHasError := false
		for _, child := range node.Children {
			childHasError = childHasError || child.HasError
		}
		if node.Kind == KindRaw || node.HasError && !childHasError {
			start, end := clampRange(source, node.Start, node.End)
			snippetEnd := min(end, start+120)
			problems = append(problems, fmt.Sprintf("%s error=%t [%d:%d] %q", path, node.HasError, start, end, source[start:snippetEnd]))
			if len(problems) >= 10 {
				return
			}
		}
		for _, child := range node.Children {
			visit(child, path)
			if len(problems) >= 10 {
				return
			}
		}
	}
	visit(root, "")
	if len(problems) != 0 {
		t.Fatalf("CST contains raw or erroneous nodes:\n%s", strings.Join(problems, "\n"))
	}
}

func readRealWorldFixtures(t *testing.T) []realWorldFixture {
	t.Helper()
	manifestPath := filepath.Join(realWorldFixtureDir, "SOURCES.tsv")
	// #nosec G304
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("open fixture manifest: %v", err)
	}

	reader := csv.NewReader(bytes.NewReader(manifest))
	reader.Comma = '\t'
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read fixture manifest: %v", err)
	}
	if len(records) == 0 || strings.Join(records[0], "\t") != "project\tpath\turl" {
		t.Fatal("fixture manifest has an invalid header")
	}

	fixtures := make([]realWorldFixture, 0, len(records)-1)
	for line, record := range records[1:] {
		if len(record) != 3 {
			t.Fatalf("fixture manifest line %d has %d fields, want 3", line+2, len(record))
		}
		if record[0] == "" || record[1] == "" || record[2] == "" {
			t.Fatalf("fixture manifest line %d contains an empty field", line+2)
		}
		if !strings.HasPrefix(record[2], "https://raw.githubusercontent.com/") {
			t.Fatalf("fixture manifest line %d has unexpected URL %q", line+2, record[2])
		}
		fixtures = append(fixtures, realWorldFixture{project: record[0], path: record[1]})
	}
	return fixtures
}
