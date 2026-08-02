package preprocess

import (
	"github.com/pawnkit/pawnkit-core/diagnostic"
	"github.com/pawnkit/pawnkit-core/source"
)

// Code is a stable, machine-readable preprocessor diagnostic identifier.
type Code string

const (
	// CodeUnterminatedConditional reports an unclosed conditional block.
	CodeUnterminatedConditional Code = "preprocess/unterminated-conditional"
	// CodeUnmatchedElseif reports an #elseif without a matching #if.
	CodeUnmatchedElseif Code = "preprocess/unmatched-elseif"
	// CodeUnmatchedElse reports an #else without a matching #if.
	CodeUnmatchedElse Code = "preprocess/unmatched-else"
	// CodeUnmatchedEndif reports an #endif without a matching #if.
	CodeUnmatchedEndif Code = "preprocess/unmatched-endif"
	// CodeConditionalDepthLimit reports a conditional nesting limit.
	CodeConditionalDepthLimit Code = "preprocess/conditional-depth-limit"
	// CodeUnresolvableCondition reports a condition that cannot be evaluated.
	CodeUnresolvableCondition Code = "preprocess/unresolvable-condition"
	// CodeUnknownDirective reports an unsupported directive.
	CodeUnknownDirective Code = "preprocess/unknown-directive"
	// CodeMalformedDefine reports an invalid macro definition.
	CodeMalformedDefine Code = "preprocess/malformed-define"
	// CodeMacroArgumentMismatch reports a macro argument count mismatch.
	CodeMacroArgumentMismatch Code = "preprocess/macro-argument-mismatch"
	// CodeUnterminatedInvocation reports an unclosed macro invocation.
	CodeUnterminatedInvocation Code = "preprocess/unterminated-macro-invocation"
	// CodeExpansionDepthLimit reports a macro expansion depth limit.
	CodeExpansionDepthLimit Code = "preprocess/expansion-depth-limit"
	// CodeOutputSizeLimit reports an expanded output size limit.
	CodeOutputSizeLimit Code = "preprocess/output-size-limit"
	// CodeMalformedInclude reports an invalid include directive.
	CodeMalformedInclude Code = "preprocess/malformed-include"
	// CodeIncludeNotFound reports a missing required include.
	CodeIncludeNotFound Code = "preprocess/include-not-found"
	// CodeIncludeCycle reports a recursive include.
	CodeIncludeCycle Code = "preprocess/include-cycle"
	// CodeIncludeDepthLimit reports an include nesting limit.
	CodeIncludeDepthLimit Code = "preprocess/include-depth-limit"
	// CodeUserError reports a source #error directive.
	CodeUserError Code = "preprocess/user-error"
	// CodeUserWarning reports a source #warning directive.
	CodeUserWarning Code = "preprocess/user-warning"
	// CodeAssertFailed reports a failed #assert directive.
	CodeAssertFailed Code = "preprocess/assert-failed"
	// CodeAssertUnknown reports an unevaluable #assert directive.
	CodeAssertUnknown Code = "preprocess/assert-unknown"
)

// ByteRange is a half-open source byte range within one file (see
// Diagnostic.File).
type ByteRange struct {
	Start int
	End   int
}

// Diagnostic is a preprocessor-stage finding. File indexes into
// Result.Files; convert to pawnkit-core's shared format with [ToCore].
type Diagnostic struct {
	File     uint32
	Code     Code
	Severity diagnostic.Severity
	Message  string
	Range    ByteRange
}

// ToCore converts d into the shared diagnostic.Diagnostic interchange
// format. file is the caller's pawnkit-core FileID for d.File; callers
// processing multiple files (via #include splicing) look up Result.Files[d.File]
// in their own source.Registry to obtain it.
func (d Diagnostic) ToCore(file source.FileID) diagnostic.Diagnostic {
	return diagnostic.New(
		"pawn-analysis:"+string(d.Code),
		"pawn-analysis",
		d.Severity,
		d.Message,
		source.Span{File: file, Start: source.Offset(d.Range.Start), End: source.Offset(d.Range.End)},
	)
}
