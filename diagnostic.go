package parser

import "github.com/pawnkit/pawn-parser/token"

// DiagnosticCode is a stable, machine-readable syntax diagnostic identifier.
type DiagnosticCode string

const (
	// DiagnosticUnexpectedToken reports a token consumed by generic recovery.
	DiagnosticUnexpectedToken DiagnosticCode = "unexpected_token"
	// DiagnosticUnexpectedDelimiter reports an unmatched closing delimiter.
	DiagnosticUnexpectedDelimiter DiagnosticCode = "unexpected_closing_delimiter"
	// DiagnosticMissingToken reports required punctuation that was absent.
	DiagnosticMissingToken DiagnosticCode = "missing_token"
	// DiagnosticMissingExpression reports an absent expression operand.
	DiagnosticMissingExpression DiagnosticCode = "missing_expression"
	// DiagnosticMissingIdentifier reports an absent required identifier.
	DiagnosticMissingIdentifier DiagnosticCode = "missing_identifier"
	// DiagnosticMissingDeclaration reports an absent declaration component.
	DiagnosticMissingDeclaration DiagnosticCode = "missing_declaration_component"
	// DiagnosticMaximumDepth reports that the parser nesting limit was reached.
	DiagnosticMaximumDepth DiagnosticCode = "maximum_parse_depth"
	// DiagnosticUnrecoverable reports that parsing could not make progress.
	DiagnosticUnrecoverable DiagnosticCode = "unrecoverable_parse_failure"
)

// ByteRange is a half-open source byte range.
type ByteRange struct {
	Start int
	End   int
}

// RecoveryKind describes the edit shape selected during recovery.
type RecoveryKind string

const (
	// RecoveryNone indicates that no directly applicable edit is available.
	RecoveryNone RecoveryKind = "none"
	// RecoveryInsert inserts replacement text at an empty recovery range.
	RecoveryInsert RecoveryKind = "insert"
	// RecoveryRemove removes the recovery range.
	RecoveryRemove RecoveryKind = "remove"
	// RecoveryReplace replaces the recovery range with replacement text.
	RecoveryReplace RecoveryKind = "replace"
)

// RecoveryConfidence states whether a recovery is safe to apply directly.
type RecoveryConfidence string

const (
	// RecoveryExact marks an unambiguous parser-selected edit.
	RecoveryExact RecoveryConfidence = "exact"
	// RecoverySuggested marks an ambiguous, non-applicable recovery hint.
	RecoverySuggested RecoveryConfidence = "suggested"
)

// Recovery describes a source edit associated with a diagnostic.
type Recovery struct {
	Kind        RecoveryKind
	Range       ByteRange
	Replacement string
	Confidence  RecoveryConfidence
}

// Diagnostic is a structured syntax error produced while building the CST.
type Diagnostic struct {
	Code     DiagnosticCode
	Message  string
	Range    ByteRange
	Found    token.Token
	Expected []token.Kind
	Recovery Recovery
}
