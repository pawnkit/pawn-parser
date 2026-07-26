package parser

import "testing"

func TestDeclarationIndex(t *testing.T) {
	t.Parallel()
	source := []byte("new value;\nenum State { Idle }\nforward Run();\nstock Run() { return value; }\n")
	index := BuildDeclarationIndex(ParseWithProfile(source, ProfileAnalysis))
	if !index.Reliable() {
		t.Fatal("index is not reliable")
	}

	want := []struct {
		kind Kind
		name string
		text string
	}{
		{KindVariableDeclaration, "value", "new value;"},
		{KindEnumDeclaration, "State", "enum State { Idle }"},
		{KindFunctionDeclaration, "Run", "forward Run();"},
		{KindFunctionDefinition, "Run", "stock Run() { return value; }"},
	}
	if index.Len() != len(want) {
		t.Fatalf("declaration count = %d, want %d", index.Len(), len(want))
	}
	for position, expected := range want {
		item, ok := index.At(position)
		if !ok {
			t.Fatalf("missing declaration %d", position)
		}
		if item.Kind != expected.kind || item.Name != expected.name {
			t.Fatalf("declaration %d = %s %q", position, item.Kind, item.Name)
		}
		if text := string(source[item.Range.Start:item.Range.End]); text != expected.text {
			t.Fatalf("declaration %d text = %q", position, text)
		}
	}
}

func TestDeclarationIdentitySurvivesBodyEdit(t *testing.T) {
	t.Parallel()
	before := BuildDeclarationIndex(ParseForLinter([]byte("stock Add() { return 1; }\n")))
	after := BuildDeclarationIndex(ParseForLinter([]byte("stock Add() { return 2; }\n")))
	first, _ := before.At(0)
	second, _ := after.At(0)
	if first.Identity != second.Identity {
		t.Fatal("identity changed after a body edit")
	}
	if first.Fingerprint == second.Fingerprint {
		t.Fatal("fingerprint did not change after a body edit")
	}
}

func TestDeclarationIdentityDistinguishesDuplicates(t *testing.T) {
	t.Parallel()
	index := BuildDeclarationIndex(ParseForLinter([]byte("stock Run() {}\nstock Run() {}\n")))
	first, _ := index.At(0)
	second, _ := index.At(1)
	if first.Identity == second.Identity {
		t.Fatal("duplicate declarations share an identity")
	}
}

func TestDeclarationIndexRejectsMalformedSource(t *testing.T) {
	t.Parallel()
	index := BuildDeclarationIndex(ParseForLinter([]byte("stock Broken( {\nstock Fine() {}\n")))
	if index.Reliable() {
		t.Fatal("malformed index is reliable")
	}
	if index.Len() == 0 {
		t.Fatal("recovery returned no boundaries")
	}
}

func TestDeclarationIndexAtBounds(t *testing.T) {
	t.Parallel()
	index := BuildDeclarationIndex(ParseForLinter([]byte("main() {}\n")))
	if _, ok := index.At(-1); ok {
		t.Fatal("negative index succeeded")
	}
	if _, ok := index.At(index.Len()); ok {
		t.Fatal("past-end index succeeded")
	}
}
