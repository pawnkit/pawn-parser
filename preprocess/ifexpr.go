package preprocess

import (
	"strconv"
	"strings"

	"github.com/pawnkit/pawn-parser/token"
)

// evalCondition evaluates the constant-expression tokens of a #if/#elseif/
// #assert directive. ok is false when evaluation encountered something this
// narrow evaluator does not support (an undefined-behaviour result must
// never be invented); callers should default to treating the branch as
// inactive and report [CodeUnresolvableCondition] in that case.
func (e *engine) evalCondition(toks []token.Token, source []byte) (value int64, ok bool) {
	if len(toks) == 0 {
		return 0, false
	}

	condition := &engine{
		ctx:           e.ctx,
		cancellable:   e.cancellable,
		macros:        e.macros,
		symbols:       e.symbols,
		opts:          e.opts,
		files:         e.files,
		listingCode:   -1,
		conditionMode: true,
	}
	condition.out = make([]token.Token, 0, len(toks))
	condition.expandedBuf = make([]byte, 0, len(source))
	condition.expandRun(&frame{}, toPtoks(source, toks), nil, 0)
	if condition.truncated || condition.cancelled != nil {
		return 0, false
	}

	p := &exprEval{
		toks:         toPtoks(condition.expandedBuf, condition.out),
		macros:       e.macros,
		symbols:      e.symbols,
		declarations: e.declarations,
	}
	v := p.parseLogicalOr()
	if p.unknown || p.pos != len(p.toks) {
		return 0, false
	}
	return v, true
}

type exprEval struct {
	toks         []ptok
	pos          int
	macros       *macroTable
	symbols      map[string]string
	declarations *declarationTracker
	guard        map[string]bool
	unknown      bool
}

func (p *exprEval) atEnd() bool { return p.pos >= len(p.toks) }

func (p *exprEval) cur() ptok {
	if p.atEnd() {
		return ptok{Token: token.Token{Kind: token.EOF}}
	}
	return p.toks[p.pos]
}

func (p *exprEval) advance() ptok {
	t := p.cur()
	if !p.atEnd() {
		p.pos++
	}
	return t
}

func (p *exprEval) at(k token.Kind) bool { return p.cur().Kind == k }

func (p *exprEval) fail() {
	p.unknown = true
}

type binOp struct {
	kind token.Kind
	eval func(a, b int64) int64
}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func (p *exprEval) parseBinary(next func() int64, ops []binOp) int64 {
	left := next()
	for {
		matched := false
		for _, op := range ops {
			if p.at(op.kind) {
				p.advance()
				right := next()
				left = op.eval(left, right)
				matched = true
				break
			}
		}
		if !matched {
			return left
		}
	}
}

func (p *exprEval) parseLogicalOr() int64 {
	return p.parseBinary(p.parseLogicalAnd, []binOp{
		{token.OrOr, func(a, b int64) int64 { return boolToInt(a != 0 || b != 0) }},
	})
}

func (p *exprEval) parseLogicalAnd() int64 {
	return p.parseBinary(p.parseBitOr, []binOp{
		{token.AndAnd, func(a, b int64) int64 { return boolToInt(a != 0 && b != 0) }},
	})
}

func (p *exprEval) parseBitOr() int64 {
	return p.parseBinary(p.parseBitXor, []binOp{
		{token.Pipe, func(a, b int64) int64 { return a | b }},
	})
}

func (p *exprEval) parseBitXor() int64 {
	return p.parseBinary(p.parseBitAnd, []binOp{
		{token.Caret, func(a, b int64) int64 { return a ^ b }},
	})
}

func (p *exprEval) parseBitAnd() int64 {
	return p.parseBinary(p.parseEquality, []binOp{
		{token.Amp, func(a, b int64) int64 { return a & b }},
	})
}

func (p *exprEval) parseEquality() int64 {
	return p.parseBinary(p.parseRelational, []binOp{
		{token.Eq, func(a, b int64) int64 { return boolToInt(a == b) }},
		{token.NotEq, func(a, b int64) int64 { return boolToInt(a != b) }},
	})
}

func (p *exprEval) parseRelational() int64 {
	return p.parseBinary(p.parseShift, []binOp{
		{token.Lt, func(a, b int64) int64 { return boolToInt(a < b) }},
		{token.Gt, func(a, b int64) int64 { return boolToInt(a > b) }},
		{token.LtEq, func(a, b int64) int64 { return boolToInt(a <= b) }},
		{token.GtEq, func(a, b int64) int64 { return boolToInt(a >= b) }},
	})
}

func (p *exprEval) parseShift() int64 {
	return p.parseBinary(p.parseAdditive, []binOp{
		{token.Shl, func(a, b int64) int64 { return a << uint64(b) }}, //nolint:gosec // #if constants are small.
		{token.Shr, func(a, b int64) int64 { return a >> uint64(b) }}, //nolint:gosec // #if constants are small.
	})
}

func (p *exprEval) parseAdditive() int64 {
	return p.parseBinary(p.parseMultiplicative, []binOp{
		{token.Plus, func(a, b int64) int64 { return a + b }},
		{token.Minus, func(a, b int64) int64 { return a - b }},
	})
}

func (p *exprEval) parseMultiplicative() int64 {
	return p.parseBinary(p.parseUnary, []binOp{
		{token.Star, func(a, b int64) int64 { return a * b }},
		{token.Slash, func(a, b int64) int64 {
			if b == 0 {
				p.fail()
				return 0
			}
			return a / b
		}},
		{token.Percent, func(a, b int64) int64 {
			if b == 0 {
				p.fail()
				return 0
			}
			return a % b
		}},
	})
}

func (p *exprEval) parseUnary() int64 {
	switch p.cur().Kind {
	case token.Bang:
		p.advance()
		return boolToInt(p.parseUnary() == 0)
	case token.Minus:
		p.advance()
		return -p.parseUnary()
	case token.Plus:
		p.advance()
		return p.parseUnary()
	case token.Tilde:
		p.advance()
		return ^p.parseUnary()
	case token.KwSizeof:
		p.advance()
		return p.parseSizeof()
	default:
		return p.parsePrimary()
	}
}

func (p *exprEval) parseSizeof() int64 {
	parenthesized := false
	if p.at(token.LParen) {
		parenthesized = true
		p.advance()
	}
	if !p.at(token.Identifier) {
		p.fail()
		return 0
	}
	name := p.advance().text
	var selectors []string
	for p.at(token.LBracket) {
		p.advance()
		selector := ""
		if p.at(token.Identifier) || p.at(token.IntLiteral) {
			selector = p.advance().text
		}
		if !p.at(token.RBracket) {
			p.fail()
			return 0
		}
		p.advance()
		selectors = append(selectors, selector)
	}
	if parenthesized {
		if !p.at(token.RParen) {
			p.fail()
			return 0
		}
		p.advance()
	}
	value, ok := p.declarations.sizeOf(name, selectors)
	if !ok {
		p.fail()
		return 0
	}
	return value
}

func (p *exprEval) parsePrimary() int64 {
	tok := p.cur()
	switch tok.Kind {
	case token.IntLiteral:
		p.advance()
		return parseIntLiteral(tok.text)
	case token.CharLiteral:
		p.advance()
		return parseCharLiteral(tok.text)
	case token.LParen:
		p.advance()
		v := p.parseLogicalOr()
		if !p.at(token.RParen) {
			p.fail()
			return 0
		}
		p.advance()
		return v
	case token.KwDefined:
		p.advance()
		return p.parseDefined()
	case token.Identifier:
		p.advance()
		return p.evalIdent(tok.text)
	default:
		p.fail()
		return 0
	}
}

func (p *exprEval) parseDefined() int64 {
	paren := false
	if p.at(token.LParen) {
		paren = true
		p.advance()
	}
	if !p.at(token.Identifier) {
		p.fail()
		return 0
	}
	name := p.advance().text
	if paren {
		if !p.at(token.RParen) {
			p.fail()
			return 0
		}
		p.advance()
	}
	_, symbol := p.symbols[name]
	declared := p.declarations != nil && p.declarations.defined(name)
	return boolToInt(p.macros.defined(name) || symbol || declared)
}

func (p *exprEval) evalIdent(name string) int64 {
	if p.macros == nil || p.guard[name] {
		return 0
	}
	m, ok := p.macros.lookup(name)
	if !ok || m.Kind != MacroObjectLike {
		var tokens []ptok
		if value, symbol := p.symbols[name]; symbol {
			tokens = tokenizeBody(value)
		} else if constant, declared := p.declarations.constant(name); declared {
			tokens = constant
		} else {
			return 0
		}
		guard := cloneExprGuard(p.guard, name)
		sub := &exprEval{toks: tokens, macros: p.macros, symbols: p.symbols, declarations: p.declarations, guard: guard}
		result := sub.parseLogicalOr()
		if sub.unknown || sub.pos != len(sub.toks) {
			p.fail()
			return 0
		}
		return result
	}
	guard := cloneExprGuard(p.guard, name)
	sub := &exprEval{toks: m.Body, macros: p.macros, symbols: p.symbols, declarations: p.declarations, guard: guard}
	v := sub.parseLogicalOr()
	if sub.unknown || sub.pos != len(sub.toks) {
		p.fail()
		return 0
	}
	return v
}

func cloneExprGuard(current map[string]bool, name string) map[string]bool {
	guard := make(map[string]bool, len(current)+1)
	for k := range current {
		guard[k] = true
	}
	guard[name] = true
	return guard
}

func parseIntLiteral(text string) int64 {
	text = strings.ReplaceAll(text, "_", "")
	v, err := strconv.ParseInt(text, 0, 64)
	if err != nil {
		uv, uerr := strconv.ParseUint(text, 0, 64)
		if uerr != nil {
			return 0
		}
		return int64(uv) //nolint:gosec // Pawn cells wrap; preserve bit pattern.
	}
	return v
}

func parseCharLiteral(text string) int64 {
	text = strings.TrimPrefix(text, "'")
	text = strings.TrimSuffix(text, "'")
	if text == "" {
		return 0
	}
	if text[0] != '\\' {
		r := []rune(text)
		if len(r) == 0 {
			return 0
		}
		return int64(r[0])
	}
	if len(text) < 2 {
		return 0
	}
	switch text[1] {
	case 'n':
		return '\n'
	case 't':
		return '\t'
	case 'r':
		return '\r'
	case '0':
		return 0
	case '\\':
		return '\\'
	case '\'':
		return '\''
	case '"':
		return '"'
	default:
		return int64(text[1])
	}
}
