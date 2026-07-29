package parser

import (
	"github.com/pawnkit/pawn-parser/token"
)

// RebaseCompactTrivia shifts a clean compact tree after a trivia-only edit.
// It returns false when the token stream could parse differently.
func RebaseCompactTrivia(
	source []byte,
	tokens []token.Token,
	previous *CompactFile,
	before ByteRange,
	after ByteRange,
) (*CompactFile, bool) {
	if previous == nil || previous.HasParseErrors() || len(previous.Lines.Starts) != 0 {
		return nil, false
	}
	if len(tokens) == 0 || tokens[len(tokens)-1].Kind != token.EOF {
		end := token.Position{Offset: len(source)}
		tokens = append(append([]token.Token(nil), tokens...), token.Token{Kind: token.EOF, Start: end, End: end})
	}
	if !sameCompactGrammar(source, tokens, previous) {
		return nil, false
	}

	compactTokens, trivia, origins, macroNames := compactTokens(tokens, ParseOptions{})
	next := *previous
	next.Source = source
	next.Tokens = compactTokens
	next.Trivia = trivia
	next.Origins = origins
	next.MacroNames = macroNames
	next.Tree.Nodes = append([]CompactNode(nil), previous.Tree.Nodes...)
	for i := range next.Tree.Nodes {
		node := &next.Tree.Nodes[i]
		node.Start = shiftedStart(node.Start, before, after)
		node.End = shiftedEnd(node.End, before, after)
		if node.TokenKind != token.Invalid {
			node.TokenStart = shiftedStart(node.TokenStart, before, after)
			node.TokenEnd = shiftedEnd(node.TokenEnd, before, after)
		}
	}
	if len(next.Tree.Nodes) != 0 {
		root := &next.Tree.Nodes[next.Tree.Root]
		root.Start = 0
		root.End = compactUint(len(source))
	}
	return &next, true
}

func sameCompactGrammar(source []byte, tokens []token.Token, previous *CompactFile) bool {
	if len(tokens) != len(previous.Tokens) {
		return false
	}
	for i, current := range tokens {
		old := previous.Tokens[i]
		if current.Kind != old.Kind ||
			current.Text(source) != compactTokenText(previous.Source, old) ||
			triviaFlags(current.LeadingTrivia) != compactTriviaFlags(previous.Trivia, old.LeadingStart, old.LeadingCount) ||
			triviaFlags(current.TrailingTrivia) != compactTriviaFlags(previous.Trivia, old.TrailingStart, old.TrailingCount) {
			return false
		}
	}
	return true
}

func compactTokenText(source []byte, item CompactToken) string {
	start, end := int(item.Start.Offset), int(item.End.Offset)
	if start < 0 || end > len(source) || start > end {
		return ""
	}
	return string(source[start:end])
}

func triviaFlags(items []token.Trivia) token.TriviaFlags {
	var flags token.TriviaFlags
	for _, item := range items {
		flags |= token.TriviaPresent
		if item.Kind == token.Newline {
			flags |= token.TriviaEndsLine
		}
	}
	return flags
}

func compactTriviaFlags(items []CompactTrivia, start, count uint32) token.TriviaFlags {
	if start > compactUint(len(items)) || count > compactUint(len(items))-start {
		return 0
	}
	var flags token.TriviaFlags
	for _, item := range items[start : start+count] {
		flags |= token.TriviaPresent
		if item.Kind == token.Newline {
			flags |= token.TriviaEndsLine
		}
	}
	return flags
}

func shiftedStart(offset uint32, before, after ByteRange) uint32 {
	value := int(offset)
	if value < before.Start {
		return offset
	}
	if value >= before.End {
		return compactUint(value + after.End - before.End)
	}
	return compactUint(after.Start + min(value-before.Start, after.End-after.Start))
}

func shiftedEnd(offset uint32, before, after ByteRange) uint32 {
	value := int(offset)
	if value <= before.Start {
		return offset
	}
	if value >= before.End {
		return compactUint(value + after.End - before.End)
	}
	return compactUint(after.Start + min(value-before.Start, after.End-after.Start))
}
