package lexer

import "github.com/pawnkit/pawn-parser/token"

func (s *scanner) scanString(start token.Position) rawToken {
	s.advance() // '"'
	for !s.atEnd() {
		c := s.peek()
		if c == '\\' {
			s.advance()
			if !s.atEnd() {
				s.advance()
			}
			continue
		}
		if c == '"' {
			s.advance()
			kind := token.StringLiteral
			if start.Offset < len(s.src) && s.src[start.Offset] == '!' {
				kind = token.PackedString
			}
			return rawToken{kind: kind, start: start, end: s.here()}
		}
		if c == '\n' {
			break
		}
		s.advance()
	}
	kind := token.Unknown
	return rawToken{kind: kind, start: start, end: s.here()}
}

func (s *scanner) scanChar(start token.Position) rawToken {
	s.advance() // '\''
	for !s.atEnd() {
		c := s.peek()
		if c == '\\' {
			s.advance()
			if !s.atEnd() {
				s.advance()
			}
			continue
		}
		if c == '\'' {
			s.advance()
			return rawToken{kind: token.CharLiteral, start: start, end: s.here()}
		}
		if c == '\n' {
			break
		}
		s.advance()
	}
	return rawToken{kind: token.Unknown, start: start, end: s.here()}
}

func (s *scanner) scanOperator(start token.Position) rawToken {
	if n, kind, ok := multiOperatorKind(s.peek(), s.peekAt(1), s.peekAt(2), s.peekAt(3)); ok {
		return s.consumeOp(start, n, kind)
	}

	kind, ok := singleOperatorKind(s.peek())
	if !ok {
		s.advance()
		return rawToken{kind: token.Unknown, start: start, end: s.here()}
	}
	s.advance()
	return rawToken{kind: kind, start: start, end: s.here()}
}

func multiOperatorKind(a, b, c, d byte) (int, token.Kind, bool) {
	switch a {
	case '>', '<':
		return shiftOrRelationalOperator(a, b, c, d)
	case '.':
		return dotOperator(b, c)
	case '+', '-':
		return incrementOrAssignmentOperator(a, b)
	case '*', '/', '%', '^':
		return assignmentOperator(a, b)
	case '&', '|':
		return logicalOrAssignmentOperator(a, b)
	case '=', '!', ':':
		return pairedOperator(a, b)
	}
	return 0, token.Invalid, false
}

func shiftOrRelationalOperator(a, b, c, d byte) (int, token.Kind, bool) {
	if a == '>' {
		switch {
		case b == '>' && c == '>' && d == '=':
			return 4, token.UshrAssign, true
		case b == '>' && c == '>':
			return 3, token.Ushr, true
		case b == '>' && c == '=':
			return 3, token.ShrAssign, true
		case b == '>':
			return 2, token.Shr, true
		case b == '=':
			return 2, token.GtEq, true
		}
	} else {
		switch {
		case b == '<' && c == '=':
			return 3, token.ShlAssign, true
		case b == '<':
			return 2, token.Shl, true
		case b == '=':
			return 2, token.LtEq, true
		}
	}
	return 0, token.Invalid, false
}

func dotOperator(b, c byte) (int, token.Kind, bool) {
	if b == '.' && c == '.' {
		return 3, token.Ellipsis, true
	}
	if b == '.' {
		return 2, token.DotDot, true
	}
	return 0, token.Invalid, false
}

func incrementOrAssignmentOperator(a, b byte) (int, token.Kind, bool) {
	if a == '+' && b == '+' {
		return 2, token.PlusPlus, true
	}
	if a == '-' && b == '-' {
		return 2, token.MinusMinus, true
	}
	return assignmentOperator(a, b)
}

func assignmentOperator(a, b byte) (int, token.Kind, bool) {
	if b != '=' {
		return 0, token.Invalid, false
	}
	switch a {
	case '+':
		return 2, token.PlusAssign, true
	case '-':
		return 2, token.MinusAssign, true
	case '*':
		return 2, token.StarAssign, true
	case '/':
		return 2, token.SlashAssign, true
	case '%':
		return 2, token.PercentAssign, true
	case '^':
		return 2, token.XorAssign, true
	}
	return 0, token.Invalid, false
}

func logicalOrAssignmentOperator(a, b byte) (int, token.Kind, bool) {
	if a == '&' && b == '&' {
		return 2, token.AndAnd, true
	}
	if a == '|' && b == '|' {
		return 2, token.OrOr, true
	}
	if a == '&' && b == '=' {
		return 2, token.AndAssign, true
	}
	if a == '|' && b == '=' {
		return 2, token.OrAssign, true
	}
	return 0, token.Invalid, false
}

func pairedOperator(a, b byte) (int, token.Kind, bool) {
	if a == '=' && b == '=' {
		return 2, token.Eq, true
	}
	if a == '!' && b == '=' {
		return 2, token.NotEq, true
	}
	if a == ':' && b == ':' {
		return 2, token.ColonColon, true
	}
	return 0, token.Invalid, false
}

func singleOperatorKind(c byte) (token.Kind, bool) {
	switch c {
	case '+':
		return token.Plus, true
	case '-':
		return token.Minus, true
	case '*':
		return token.Star, true
	case '/':
		return token.Slash, true
	case '%':
		return token.Percent, true
	case '=':
		return token.Assign, true
	case '<':
		return token.Lt, true
	case '>':
		return token.Gt, true
	case '&':
		return token.Amp, true
	case '|':
		return token.Pipe, true
	case '^':
		return token.Caret, true
	case '~':
		return token.Tilde, true
	case '!':
		return token.Bang, true
	case '?':
		return token.Question, true
	case ':':
		return token.Colon, true
	case ';':
		return token.Semicolon, true
	case ',':
		return token.Comma, true
	case '.':
		return token.Dot, true
	case '(':
		return token.LParen, true
	case ')':
		return token.RParen, true
	case '[':
		return token.LBracket, true
	case ']':
		return token.RBracket, true
	case '{':
		return token.LBrace, true
	case '}':
		return token.RBrace, true
	case '#':
		return token.Hash, true
	case '@':
		return token.At, true
	default:
		return token.Unknown, false
	}
}

func (s *scanner) consumeOp(start token.Position, n int, kind token.Kind) rawToken {
	for range n {
		s.advance()
	}
	return rawToken{kind: kind, start: start, end: s.here()}
}
