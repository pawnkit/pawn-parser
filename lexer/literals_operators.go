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

type operatorSpec struct {
	text string
	kind token.Kind
}

var multiCharOperators = []operatorSpec{
	{">>>=", token.UshrAssign},
	{"...", token.Ellipsis},
	{"<<=", token.ShlAssign},
	{">>=", token.ShrAssign},
	{">>>", token.Ushr},
	{"..", token.DotDot},
	{"+=", token.PlusAssign},
	{"-=", token.MinusAssign},
	{"*=", token.StarAssign},
	{"/=", token.SlashAssign},
	{"%=", token.PercentAssign},
	{"&=", token.AndAssign},
	{"|=", token.OrAssign},
	{"^=", token.XorAssign},
	{"==", token.Eq},
	{"!=", token.NotEq},
	{"<=", token.LtEq},
	{">=", token.GtEq},
	{"<<", token.Shl},
	{">>", token.Shr},
	{"&&", token.AndAnd},
	{"||", token.OrOr},
	{"++", token.PlusPlus},
	{"--", token.MinusMinus},
	{"::", token.ColonColon},
}

func (s *scanner) scanOperator(start token.Position) rawToken {
	for _, op := range multiCharOperators {
		if s.matches(op.text) {
			return s.consumeOp(start, len(op.text), op.kind)
		}
	}

	kind, ok := singleOperatorKind(s.peek())
	if !ok {
		s.advance()
		return rawToken{kind: token.Unknown, start: start, end: s.here()}
	}
	s.advance()
	return rawToken{kind: kind, start: start, end: s.here()}
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

func (s *scanner) matches(text string) bool {
	for i := range text {
		if s.peekAt(i) != text[i] {
			return false
		}
	}
	return true
}

func (s *scanner) consumeOp(start token.Position, n int, kind token.Kind) rawToken {
	for range n {
		s.advance()
	}
	return rawToken{kind: kind, start: start, end: s.here()}
}
