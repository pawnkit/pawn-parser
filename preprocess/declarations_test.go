package preprocess

import (
	"testing"

	"github.com/pawnkit/pawn-parser/lexer"
)

func TestDeclarationTrackerRecordsEnumIndexedArrays(t *testing.T) {
	t.Parallel()

	source := []byte(`enum Layout
{
    LayoutEntry[3],
    LayoutAfter
}
static stock const values[Layout];
`)
	tracker := newDeclarationTracker(defaultMaxSymbolLength, nil, nil, false)
	for _, item := range toPtoks(source, lexer.Tokenize(source)) {
		tracker.observe(item)
	}

	if got := tracker.enumSizes["Layout"]; got != 4 {
		t.Fatalf("enum size = %d, want 4 (fields: %#v)", got, tracker.enumFields)
	}
	array, ok := tracker.arrays["values"]
	if !ok {
		t.Fatalf("values array was not recorded (arrays: %#v)", tracker.arrays)
	}
	if array.enumIndex != "Layout" {
		t.Fatalf("enum index = %q, want Layout", array.enumIndex)
	}
	if got, ok := tracker.sizeOf("values", []string{"LayoutEntry"}); !ok || got != 3 {
		t.Fatalf("sizeof values[LayoutEntry] = %d, %t; want 3, true", got, ok)
	}
}
