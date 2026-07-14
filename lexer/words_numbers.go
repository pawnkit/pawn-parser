package lexer

import "github.com/pawnkit/pawn-parser/token"

func isIdentStart(c byte) bool {
	return c == '_' || c == '@' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentChar(c byte) bool {
	return isIdentStart(c) || isDigit(c)
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isMacroParamChar(c byte) bool {
	return isDigit(c) || c == '%'
}

func (s *scanner) scanIdentifier(start token.Position) rawToken {
	for isIdentChar(s.peek()) {
		s.advance()
	}
	end := s.here()
	if kw, ok := token.LookupKeyword(string(s.src[start.Offset:end.Offset])); ok {
		return rawToken{kind: kw, start: start, end: end}
	}
	return rawToken{kind: token.Identifier, start: start, end: end}
}

func (s *scanner) scanMacroParam(start token.Position) rawToken {
	s.advance()
	if s.peek() == '%' {
		s.advance()
		return rawToken{kind: token.MacroParam, start: start, end: s.here()}
	}
	for isDigit(s.peek()) {
		s.advance()
	}
	return rawToken{kind: token.MacroParam, start: start, end: s.here()}
}

func (s *scanner) scanNumber(start token.Position) rawToken {
	isFloat := false
	if s.peek() == '0' && (s.peekAt(1) == 'x' || s.peekAt(1) == 'X') {
		s.advance()
		s.advance()
		s.scanDigitsWithSeparators(isHexDigit)
		return rawToken{kind: token.IntLiteral, start: start, end: s.here()}
	}
	if s.peek() == '0' && (s.peekAt(1) == 'b' || s.peekAt(1) == 'B') {
		s.advance()
		s.advance()
		s.scanDigitsWithSeparators(func(c byte) bool { return c == '0' || c == '1' })
		return rawToken{kind: token.IntLiteral, start: start, end: s.here()}
	}
	s.scanDigitsWithSeparators(isDigit)
	if s.peek() == '.' && isDigit(s.peekAt(1)) {
		isFloat = true
		s.advance()
		s.scanDigitsWithSeparators(isDigit)
	}
	if s.peek() == 'e' || s.peek() == 'E' {
		la := 1
		if s.peekAt(1) == '+' || s.peekAt(1) == '-' {
			la = 2
		}
		if isDigit(s.peekAt(la)) {
			isFloat = true
			for range la {
				s.advance()
			}
			s.scanDigitsWithSeparators(isDigit)
		}
	}
	if isFloat {
		return rawToken{kind: token.FloatLiteral, start: start, end: s.here()}
	}
	return rawToken{kind: token.IntLiteral, start: start, end: s.here()}
}

func (s *scanner) scanDigitsWithSeparators(validDigit func(byte) bool) {
	for validDigit(s.peek()) || s.peek() == '_' && validDigit(s.peekAt(1)) {
		s.advance()
	}
}

func isHexDigit(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
