package parser

import "testing"

func TestFileSet(t *testing.T) {
	var files FileSet
	data := []byte("main() {}")
	first := files.Add("main.pwn", data)
	second := files.Add("include/core.inc", []byte("native print();"))

	data[0] = 'x'
	if first.ID != 1 || second.ID != 2 {
		t.Fatalf("file IDs = %d, %d; want 1, 2", first.ID, second.ID)
	}
	if got, ok := files.File(first.ID); !ok || string(got.Data) != "main() {}" {
		t.Fatalf("File(%d) = %#v, %v", first.ID, got, ok)
	}
	if _, ok := files.File(0); ok {
		t.Fatal("File(0) unexpectedly succeeded")
	}
	if _, err := files.Require(3); err == nil {
		t.Fatal("Require(3) unexpectedly succeeded")
	}

	list := files.Files()
	list[0] = SourceFile{}
	if got, _ := files.File(first.ID); got.Name != "main.pwn" {
		t.Fatalf("Files returned aliased storage: first name = %q", got.Name)
	}
}
