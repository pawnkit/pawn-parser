package token

// CompactToken stores one token with indexed trivia and origin data.
type CompactToken struct {
	Kind Kind

	Start CompactPosition
	End   CompactPosition

	LeadingStart  uint32
	LeadingCount  uint32
	TrailingStart uint32
	TrailingCount uint32
	Origin        uint32

	LeadingFlags  TriviaFlags
	TrailingFlags TriviaFlags
}

// SyntaxToken stores the token data needed by the grammar.
type SyntaxToken struct {
	Kind          Kind
	Start         uint32
	End           uint32
	LeadingFlags  TriviaFlags
	TrailingFlags TriviaFlags
}

// TriviaFlags summarizes trivia without retaining its spans.
type TriviaFlags uint8

const (
	// TriviaPresent indicates that trivia exists.
	TriviaPresent TriviaFlags = 1 << iota
	// TriviaEndsLine indicates that trivia contains a newline.
	TriviaEndsLine
)

// CompactTrivia stores one trivia span.
type CompactTrivia struct {
	Kind  Kind
	Start CompactPosition
	End   CompactPosition
}

// CompactPosition stores a source position.
type CompactPosition struct {
	Offset uint32
	Line   uint32
	Col    uint32
}

// CompactOrigin stores one origin link.
type CompactOrigin struct {
	File   uint32
	Start  CompactPosition
	End    CompactPosition
	Macro  uint32
	Parent uint32
}
