package parser

import "testing"

func TestTypedSyntaxTraversal(t *testing.T) {
	t.Parallel()
	source := []byte("stock Add(value, other = 1) {\n    if (value) return Call(other);\n}\n")
	file := ParseWithProfile(source, ProfileAnalysis)
	root := file.Syntax()
	if !root.Valid() || root.Kind() != KindSourceFile || string(root.Bytes()) != string(source) {
		t.Fatalf("invalid syntax root: %v %s", root.Valid(), root.Kind())
	}

	declarations := root.Declarations()
	if !declarations.Next() {
		t.Fatal("missing function declaration")
	}
	function, ok := AsFunction(declarations.Declaration())
	if !ok {
		t.Fatalf("declaration kind = %s", declarations.Declaration().Kind())
	}
	name, ok := function.Name()
	if !ok || name.Token().Text() != "Add" {
		t.Fatalf("function name = %q", name.Text())
	}
	parameters := function.Parameters()
	count := 0
	for parameters.Next() {
		parameterName, present := parameters.Parameter().Name()
		if !present || !parameterName.Valid() {
			t.Fatal("parameter has no name")
		}
		count++
	}
	if count != 2 {
		t.Fatalf("parameter count = %d, want 2", count)
	}
	body, ok := function.Body()
	if !ok {
		t.Fatal("function has no body")
	}
	statements := body.Statements()
	if !statements.Next() {
		t.Fatal("function body has no statements")
	}
	ifStatement, ok := AsIf(statements.Node())
	if !ok {
		t.Fatalf("statement kind = %s", statements.Node().Kind())
	}
	if _, ok := ifStatement.Condition(); !ok {
		t.Fatal("if statement has no condition")
	}
}

func TestSyntaxIterationDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun cannot run in parallel.
	file := ParseWithProfile([]byte("main() { return; }\n"), ProfileAnalysis)
	root := file.Syntax()
	allocations := testing.AllocsPerRun(100, func() {
		iterator := root.Children()
		for iterator.Next() {
			_ = iterator.Node().Kind()
		}
	})
	if allocations != 0 {
		t.Fatalf("syntax iteration allocated %.1f objects", allocations)
	}
}

func TestLosslessSyntaxTokenTrivia(t *testing.T) {
	t.Parallel()
	file := ParseWithProfile([]byte("main /* note */ () { return; }\n"), ProfileLossless)
	declarations := file.Syntax().Declarations()
	if !declarations.Next() {
		t.Fatal("missing function")
	}
	function, ok := AsFunction(declarations.Declaration())
	if !ok {
		t.Fatal("declaration is not a function")
	}
	name, ok := function.Name()
	if !ok || len(name.Token().TrailingTrivia()) == 0 {
		t.Fatal("lossless token omitted trailing trivia")
	}
}
