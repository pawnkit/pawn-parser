package parser

import (
	"fmt"

	"github.com/pawnkit/pawnkit-core/diagnostic"
	"github.com/pawnkit/pawnkit-core/source"
	"github.com/pawnkit/pawnkit-core/textedit"
)

// FileID is an alias so FileIDs from FileSet interoperate with pawnkit-core
// (source.FileID and parser.FileID have always shared the same uint32
// shape; this makes it official rather than coincidental).
type FileID = source.FileID

// Span converts r into a pawnkit-core source.Span for file.
func (r ByteRange) Span(file source.FileID) source.Span {
	return source.Span{File: file, Start: source.Offset(r.Start), End: source.Offset(r.End)}
}

// ByteRangeFromSpan drops s.File, keeping only the byte offsets.
func ByteRangeFromSpan(s source.Span) ByteRange {
	return ByteRange{Start: int(s.Start), End: int(s.End)}
}

// ToCore converts d into the shared diagnostic.Diagnostic interchange
// format used across PawnKit tools (CLI JSON, SARIF, editor protocols).
// Severity is always SeverityError: the parser does not yet classify
// diagnostics by severity.
func (d Diagnostic) ToCore(file source.FileID) diagnostic.Diagnostic {
	cd := diagnostic.New(
		"pawn-parser:"+string(d.Code),
		"pawn-parser",
		diagnostic.SeverityError,
		d.Message,
		d.Range.Span(file),
	)

	if fix, kind, ok := d.Recovery.toFix(file); ok {
		switch kind {
		case diagnostic.FixSafe:
			cd.SafeFixes = []diagnostic.Fix{fix}
		case diagnostic.FixReviewRequired:
			cd.ReviewFixes = []diagnostic.Fix{fix}
		}
	}

	return cd
}

func (r Recovery) toFix(file source.FileID) (diagnostic.Fix, diagnostic.FixKind, bool) {
	switch r.Kind {
	case RecoveryInsert, RecoveryRemove, RecoveryReplace:
	default:
		return diagnostic.Fix{}, 0, false
	}

	kind := diagnostic.FixReviewRequired
	if r.Confidence == RecoveryExact {
		kind = diagnostic.FixSafe
	}

	edit := textedit.Edit{Span: r.Range.Span(file), NewText: r.Replacement}
	we := textedit.WorkspaceEdit{Documents: []textedit.DocumentEdit{
		{File: file, Version: textedit.AnyVersion, Edits: []textedit.Edit{edit}},
	}}

	var msg string
	switch r.Kind {
	case RecoveryInsert:
		msg = fmt.Sprintf("insert %q", r.Replacement)
	case RecoveryRemove:
		msg = "remove"
	case RecoveryReplace:
		msg = fmt.Sprintf("replace with %q", r.Replacement)
	default:
		msg = string(r.Kind)
	}

	return diagnostic.Fix{Message: msg, Kind: kind, Edit: we}, kind, true
}
