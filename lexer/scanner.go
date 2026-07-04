package lexer

import "github.com/pawnkit/pawn-parser/token"

type rawToken struct {
	kind  token.Kind
	start token.Position
	end   token.Position
}

type scanner struct {
	src  []byte
	pos  int
	line int
	col  int
}

func newScanner(src []byte) *scanner {
	return &scanner{src: src, pos: 0, line: 1, col: 1}
}

func (s *scanner) here() token.Position {
	return token.Position{Offset: s.pos, Line: s.line, Col: s.col}
}

func (s *scanner) atEnd() bool {
	return s.pos >= len(s.src)
}

func (s *scanner) peek() byte {
	if s.pos >= len(s.src) {
		return 0
	}
	return s.src[s.pos]
}

func (s *scanner) peekAt(offset int) byte {
	idx := s.pos + offset
	if idx >= len(s.src) || idx < 0 {
		return 0
	}
	return s.src[idx]
}

func (s *scanner) advance() byte {
	c := s.src[s.pos]
	s.pos++
	if c == '\n' {
		s.line++
		s.col = 1
	} else {
		s.col++
	}
	return c
}

func (s *scanner) nextRaw() rawToken {
	start := s.here()
	if s.atEnd() {
		return rawToken{kind: token.EOF, start: start, end: start}
	}

	c := s.peek()

	switch {
	case c == ' ' || c == '\t' || c == '\r':
		return s.scanWhitespace(start)
	case c == '\n':
		s.advance()
		return rawToken{kind: token.Newline, start: start, end: s.here()}
	case c == '\\' && (s.peekAt(1) == '\n' || (s.peekAt(1) == '\r' && s.peekAt(2) == '\n')):
		s.advance()
		if s.peek() == '\r' {
			s.advance()
		}
		if s.peek() == '\n' {
			s.advance()
		}
		return rawToken{kind: token.LineContinuation, start: start, end: s.here()}
	case c == '/' && s.peekAt(1) == '/':
		return s.scanLineComment(start)
	case c == '/' && s.peekAt(1) == '*':
		return s.scanBlockComment(start)
	case isIdentStart(c):
		return s.scanIdentifier(start)
	case isDigit(c):
		return s.scanNumber(start)
	case c == '.' && isDigit(s.peekAt(1)):
		return s.scanNumber(start)
	case c == '"':
		return s.scanString(start)
	case c == '\'':
		return s.scanChar(start)
	case c == '!' && s.peekAt(1) == '"':
		s.advance()
		return s.scanString(start)
	case c == '%' && isMacroParamChar(s.peekAt(1)):
		return s.scanMacroParam(start)
	default:
		return s.scanOperator(start)
	}
}

func (s *scanner) scanWhitespace(start token.Position) rawToken {
	for {
		c := s.peek()
		if c == ' ' || c == '\t' || c == '\r' {
			s.advance()
			continue
		}
		break
	}
	return rawToken{kind: token.Whitespace, start: start, end: s.here()}
}

func (s *scanner) scanLineComment(start token.Position) rawToken {
	s.advance() // '/'
	s.advance() // '/'
	for !s.atEnd() && s.peek() != '\n' && s.peek() != '\r' {
		s.advance()
	}
	return rawToken{kind: token.Comment, start: start, end: s.here()}
}

func (s *scanner) scanBlockComment(start token.Position) rawToken {
	s.advance() // '/'
	s.advance() // '*'
	for !s.atEnd() {
		if s.peek() == '*' && s.peekAt(1) == '/' {
			s.advance()
			s.advance()
			return rawToken{kind: token.Comment, start: start, end: s.here()}
		}
		s.advance()
	}
	return rawToken{kind: token.Comment, start: start, end: s.here()}
}
