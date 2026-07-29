package parser_test

import (
	"context"

	parser "github.com/pawnkit/pawn-parser"
	"github.com/pawnkit/pawn-parser/lexer"
	"github.com/pawnkit/pawn-parser/token"
)

var (
	_ func([]byte) *parser.File                                                                                        = parser.Parse
	_ func([]byte, parser.Profile) *parser.CompactFile                                                                 = parser.ParseWithProfile
	_ func([]byte, []token.Token, parser.ParseOptions) *parser.File                                                    = parser.ParseTokensWithOptions
	_ func(context.Context, []byte, []token.Token, parser.ParseOptions) (*parser.CompactFile, error)                   = parser.ParseTokensCompactContext
	_ func([]byte, []token.Token, *parser.CompactFile, parser.ByteRange, parser.ByteRange) (*parser.CompactFile, bool) = parser.RebaseCompactTrivia
	_ func([]byte) []token.Token                                                                                       = lexer.Tokenize
	_ func(context.Context, []byte) ([]token.Token, error)                                                             = lexer.TokenizeContext
	_ func(string) (token.Kind, bool)                                                                                  = token.LookupKeyword
	_ parser.Kind                                                                                                      = parser.KindInvalid
	_ parser.DiagnosticCode                                                                                            = parser.DiagnosticUnexpectedToken
	_ parser.DeclarationIndex                                                                                          = parser.BuildDeclarationIndex(nil)
	_ token.Kind                                                                                                       = token.Invalid
)

var _ = func(file *parser.CompactFile, diagnostic parser.Diagnostic, span token.Span) {
	_ = file.Tree
	_ = diagnostic.Code
	_ = diagnostic.Message
	index := parser.BuildDeclarationIndex(file)
	_, _ = index.At(0)
	_ = index.Len()
	_ = index.Reliable()
	_ = span.Start
	_ = span.End
}
