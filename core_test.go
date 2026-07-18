package parser

import (
	"testing"

	"github.com/pawnkit/pawnkit-core/diagnostic"
	"github.com/pawnkit/pawnkit-core/source"
)

func TestByteRangeSpanRoundTrip(t *testing.T) {
	t.Parallel()

	r := ByteRange{Start: 3, End: 9}
	file := source.FileID(1)

	span := r.Span(file)
	if span.File != file || span.Start != 3 || span.End != 9 {
		t.Fatalf("Span() = %+v", span)
	}

	if got := ByteRangeFromSpan(span); got != r {
		t.Fatalf("ByteRangeFromSpan() = %+v, want %+v", got, r)
	}
}

func TestDiagnosticToCoreSafeFix(t *testing.T) {
	t.Parallel()

	file := source.FileID(1)
	d := Diagnostic{
		Code:    DiagnosticMissingToken,
		Message: "expected ';'",
		Range:   ByteRange{Start: 10, End: 10},
		Recovery: Recovery{
			Kind:        RecoveryInsert,
			Range:       ByteRange{Start: 10, End: 10},
			Replacement: ";",
			Confidence:  RecoveryExact,
		},
	}

	cd := d.ToCore(file)

	if cd.Code != "pawn-parser:missing_token" {
		t.Errorf("Code = %q", cd.Code)
	}
	if cd.Severity != diagnostic.SeverityError {
		t.Errorf("Severity = %v", cd.Severity)
	}
	if len(cd.SafeFixes) != 1 || len(cd.ReviewFixes) != 0 {
		t.Fatalf("SafeFixes = %d, ReviewFixes = %d", len(cd.SafeFixes), len(cd.ReviewFixes))
	}
	if err := cd.Validate(); err != nil {
		t.Errorf("Validate() = %v", err)
	}
}

func TestDiagnosticToCoreReviewFix(t *testing.T) {
	t.Parallel()

	file := source.FileID(1)
	d := Diagnostic{
		Code:    DiagnosticSyntaxError,
		Message: "unexpected token",
		Range:   ByteRange{Start: 4, End: 6},
		Recovery: Recovery{
			Kind:       RecoveryRemove,
			Range:      ByteRange{Start: 4, End: 6},
			Confidence: RecoverySuggested,
		},
	}

	cd := d.ToCore(file)

	if len(cd.ReviewFixes) != 1 || len(cd.SafeFixes) != 0 {
		t.Fatalf("SafeFixes = %d, ReviewFixes = %d", len(cd.SafeFixes), len(cd.ReviewFixes))
	}
}

func TestDiagnosticToCoreNoRecovery(t *testing.T) {
	t.Parallel()

	file := source.FileID(1)
	d := Diagnostic{
		Code:    DiagnosticUnrecoverable,
		Message: "cannot recover",
		Range:   ByteRange{Start: 0, End: 1},
	}

	cd := d.ToCore(file)

	if len(cd.SafeFixes) != 0 || len(cd.ReviewFixes) != 0 {
		t.Fatalf("expected no fixes, got SafeFixes=%d ReviewFixes=%d", len(cd.SafeFixes), len(cd.ReviewFixes))
	}
	if err := cd.Validate(); err != nil {
		t.Errorf("Validate() = %v", err)
	}
}
