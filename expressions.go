package parser

import "github.com/pawnkit/pawn-parser/token"

const (
	bpNone = iota
	bpComma
	bpAssign
	bpTernary
	bpLogicalOr
	bpLogicalAnd
	bpBitOr
	bpBitXor
	bpBitAnd
	bpEquality
	bpRelational
	bpShift
	bpAdditive
	bpMultiplicative
	bpUnary
	bpPostfix
)

func binaryBindingPower(k token.Kind) (int, bool) {
	switch k {
	case token.OrOr:
		return bpLogicalOr, true
	case token.AndAnd:
		return bpLogicalAnd, true
	case token.Pipe:
		return bpBitOr, true
	case token.Caret:
		return bpBitXor, true
	case token.Amp:
		return bpBitAnd, true
	case token.Eq, token.NotEq:
		return bpEquality, true
	case token.Lt, token.Gt, token.LtEq, token.GtEq:
		return bpRelational, true
	case token.Shl, token.Shr, token.Ushr:
		return bpShift, true
	case token.Plus, token.Minus:
		return bpAdditive, true
	case token.Star, token.Slash, token.Percent:
		return bpMultiplicative, true
	default:
		return 0, false
	}
}

func isAssignOp(k token.Kind) bool {
	switch k {
	case token.Assign, token.PlusAssign, token.MinusAssign, token.StarAssign, token.SlashAssign,
		token.PercentAssign, token.ShlAssign, token.ShrAssign, token.UshrAssign, token.AndAssign, token.OrAssign, token.XorAssign:
		return true
	default:
		return false
	}
}

func (p *parser[N, S]) parseExpression() N {
	first := p.parseAssignment()
	if !p.at(token.Comma) {
		return first
	}
	list := p.sink.Store(Node{Kind: KindExpressionList, Start: p.sink.Start(first), Leading: p.sink.Leading(first)})
	p.sink.AddChild(list, first)
	for p.at(token.Comma) {
		p.advance()
		p.sink.AddChild(list, p.parseAssignment())
	}
	return list
}

func (p *parser[N, S]) parseAssignment() N {
	left := p.parseTernary()
	if isAssignOp(p.curKind()) {
		opTok := p.advance()
		right := p.parseAssignment()
		node := p.sink.NewNode(KindAssignmentExpression, left, right)
		p.sink.SetField(node, fieldLeft, left)
		p.sink.SetField(node, fieldRight, right)
		p.sink.SetToken(node, opTok)
		return node
	}
	return left
}

func (p *parser[N, S]) parseTernary() N {
	cond := p.parseBinary(bpLogicalOr)
	if !p.at(token.Question) {
		return cond
	}
	p.advance()
	savedSuppressTagCast := p.suppressTagCast
	p.suppressTagCast = true
	consequence := p.parseAssignment()
	p.suppressTagCast = savedSuppressTagCast
	if !p.at(token.Colon) {
		node := p.sink.NewNode(KindTernaryExpression, cond, consequence)
		p.sink.SetHasError(node, true)
		return node
	}
	p.advance()
	alternative := p.parseAssignment()
	node := p.sink.NewNode(KindTernaryExpression, cond, consequence, alternative)
	p.sink.SetField(node, fieldCondition, cond)
	p.sink.SetField(node, fieldConsequence, consequence)
	p.sink.SetField(node, fieldAlternative, alternative)
	return node
}

func (p *parser[N, S]) parseBinary(minBP int) N {
	left := p.parseUnary()
	for {
		bp, ok := binaryBindingPower(p.curKind())
		if !ok || bp < minBP {
			return left
		}
		opTok := p.advance()
		right := p.parseBinary(bp + 1)
		node := p.sink.NewNode(KindBinaryExpression, left, right)
		p.sink.SetField(node, fieldLeft, left)
		p.sink.SetField(node, fieldRight, right)
		p.sink.SetToken(node, opTok)
		left = node
	}
}

func isUnaryOp(k token.Kind) bool {
	switch k {
	case token.Bang, token.Tilde, token.Minus, token.Plus, token.PlusPlus, token.MinusMinus:
		return true
	default:
		return false
	}
}

func (p *parser[N, S]) parseUnary() N {
	if !p.enterDepth() {
		defer p.exitDepth()
		p.broken = true
		tok := p.cur()
		n := p.sink.NewLeaf(KindLiteral, tok)
		p.sink.SetHasError(n, true)
		return n
	}
	defer p.exitDepth()

	if p.isMacroUnaryOperator() {
		opTok := p.advance()
		operand := p.parseUnary()
		node := p.sink.NewNode(KindUnaryExpression, operand)
		p.sink.SetField(node, fieldExpression, operand)
		p.sink.SetToken(node, opTok)
		p.sink.SetStart(node, opTok.Start.Offset)
		p.sink.SetLeading(node, opTok.LeadingTrivia)
		return node
	}
	if isUnaryOp(p.curKind()) {
		opTok := p.advance()
		operand := p.parseUnary()
		node := p.sink.NewNode(KindUnaryExpression, operand)
		p.sink.SetField(node, fieldExpression, operand)
		p.sink.SetToken(node, opTok)
		p.sink.SetStart(node, opTok.Start.Offset)
		p.sink.SetLeading(node, opTok.LeadingTrivia)
		return node
	}
	if p.at(token.KwSizeof) {
		return p.parseSizeofLike(KindSizeofExpression)
	}
	if p.at(token.KwTagof) {
		return p.parseSizeofLike(KindTagofExpression)
	}
	if p.at(token.KwDefined) {
		return p.parseDefinedExpression()
	}
	if isTagCastStart(p) {
		return p.parseTaggedExpression()
	}
	return p.parsePostfix()
}

func (p *parser[N, S]) isMacroUnaryOperator() bool {
	return p.at(token.Identifier) && p.peekKind(1) == token.Identifier
}

func isTagCastStart[N comparable, S nodeSink[N]](p *parser[N, S]) bool {
	if p.macroTagPrefixStart() || p.genericTagPrefixStart() {
		return true
	}
	if p.curKind() != token.Identifier || p.peekKind(1) != token.Colon {
		return false
	}
	return !p.suppressTagCast || p.knowsTag(p.cur().Text(p.source))
}

func (p *parser[N, S]) parseTaggedExpression() N {
	var tag N
	var colon token.Token
	switch {
	case p.genericTagPrefixStart():
		tag = p.parseOptionalTagPrefix()
		colon = p.toks.at(p.pos - 1)
	case p.macroTagPrefixStart():
		name := p.sink.NewLeaf(KindIdentifier, p.advance())
		tag = p.parseCall(name)
		colon = p.advance()
	default:
		tag = p.sink.NewLeaf(KindIdentifier, p.advance())
		colon = p.advance()
	}
	if p.at(token.RParen) || p.at(token.Comma) || p.at(token.Semicolon) {
		node := p.sink.NewNode(KindTaggedExpression, tag)
		p.sink.SetField(node, fieldTag, tag)
		p.sink.SetEnd(node, colon.End.Offset)
		p.sink.SetTrailing(node, colon.TrailingTrivia)
		return node
	}
	operand := p.parseUnary()
	node := p.sink.NewNode(KindTaggedExpression, tag, operand)
	p.sink.SetField(node, fieldTag, tag)
	p.sink.SetField(node, fieldExpression, operand)
	return node
}

func (p *parser[N, S]) parseSizeofLike(kind Kind) N {
	kwTok := p.advance()
	if p.at(token.LParen) {
		p.advance()
		inner := p.parseExpression()
		node := p.sink.NewNode(kind, inner)
		p.sink.SetField(node, fieldExpression, inner)
		p.sink.SetToken(node, kwTok)
		p.sink.SetStart(node, kwTok.Start.Offset)
		p.sink.SetLeading(node, kwTok.LeadingTrivia)
		if p.at(token.RParen) {
			rp := p.advance()
			p.sink.SetEnd(node, rp.End.Offset)
			p.sink.SetTrailing(node, rp.TrailingTrivia)
		} else {
			p.sink.SetHasError(node, true)
		}
		return node
	}

	if !p.at(token.Identifier) {
		leaf := p.sink.NewLeaf(KindIdentifier, kwTok)
		p.sink.SetHasError(leaf, true)
		return leaf
	}
	operand := p.parsePrimary()
	for p.at(token.LBracket) {
		operand = p.parseSubscript(operand)
	}
	node := p.sink.NewNode(kind, operand)
	p.sink.SetField(node, fieldExpression, operand)
	p.sink.SetToken(node, kwTok)
	p.sink.SetStart(node, kwTok.Start.Offset)
	p.sink.SetLeading(node, kwTok.LeadingTrivia)
	return node
}

func (p *parser[N, S]) parseDefinedExpression() N {
	kwTok := p.advance()
	if !p.at(token.LParen) {
		if p.at(token.Identifier) {
			name := p.sink.NewLeaf(KindIdentifier, p.advance())
			node := p.sink.NewNode(KindDefinedExpression, name)
			p.sink.SetField(node, fieldName, name)
			p.sink.SetToken(node, kwTok)
			return node
		}
		leaf := p.sink.NewLeaf(KindIdentifier, kwTok)
		p.sink.SetHasError(leaf, true)
		return leaf
	}
	p.advance()
	var name N
	if p.at(token.Identifier) {
		name = p.sink.NewLeaf(KindIdentifier, p.advance())
	}
	node := p.sink.NewNode(KindDefinedExpression, name)
	p.sink.SetField(node, fieldName, name)
	p.sink.SetToken(node, kwTok)
	if p.at(token.RParen) {
		rp := p.advance()
		p.sink.SetEnd(node, rp.End.Offset)
		p.sink.SetTrailing(node, rp.TrailingTrivia)
	} else {
		p.sink.SetHasError(node, true)
	}
	return node
}
