package parser

import "github.com/pawnkit/pawn-parser/lexer"

// Profile selects retained syntax for a consumer.
type Profile uint8

const (
	// ProfileLossless retains tokens, trivia, origins, and syntax.
	ProfileLossless Profile = iota
	// ProfileAnalysis retains compact syntax and diagnostics.
	ProfileAnalysis
	// ProfileTokensOnly retains tokens and trivia without building syntax.
	ProfileTokensOnly
)

// ParseWithProfile parses source using a consumer-oriented retention profile.
func ParseWithProfile(source []byte, profile Profile) *CompactFile {
	switch profile {
	case ProfileLossless:
		file := ParseCompact(source, ParseOptions{})
		file.Profile = profile
		return file
	case ProfileAnalysis:
		return ParseForLinter(source)
	case ProfileTokensOnly:
		tokens, trivia := lexer.TokenizeCompactOnly(source, true)
		return &CompactFile{
			Source: source, Tokens: tokens, Trivia: trivia,
			Origins: []CompactOrigin{{}}, MacroNames: []string{""}, Profile: profile,
		}
	default:
		panic("unknown parser profile")
	}
}
