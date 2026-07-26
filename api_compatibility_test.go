package parser_test

import (
	parser "github.com/pawnkit/pawn-parser"
	"github.com/pawnkit/pawn-parser/lexer"
	"github.com/pawnkit/pawn-parser/token"
)

var (
	_ func([]byte) *parser.File                                     = parser.Parse
	_ func([]byte, parser.Profile) *parser.CompactFile              = parser.ParseWithProfile
	_ func([]byte, []token.Token, parser.ParseOptions) *parser.File = parser.ParseTokensWithOptions
	_ func([]byte) []token.Token                                    = lexer.Tokenize
	_ func(string) (token.Kind, bool)                               = token.LookupKeyword
	_ parser.Kind                                                   = parser.KindInvalid
	_ parser.DiagnosticCode                                         = parser.DiagnosticUnexpectedToken
	_ token.Kind                                                    = token.Invalid
)

var _ = func(file *parser.CompactFile, diagnostic parser.Diagnostic, span token.Span) {
	_ = file.Tree
	_ = diagnostic.Code
	_ = diagnostic.Message
	_ = span.Start
	_ = span.End
}
