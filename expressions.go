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

func (p *parser) parseExpression() *Node {
	first := p.parseAssignment()
	if !p.at(token.Comma) {
		return first
	}
	list := &Node{Kind: KindExpressionList, Start: first.Start, Leading: first.Leading}
	list.addChild(first)
	for p.at(token.Comma) {
		p.advance()
		list.addChild(p.parseAssignment())
	}
	return list
}

func (p *parser) parseAssignment() *Node {
	left := p.parseTernary()
	if isAssignOp(p.cur().Kind) {
		opTok := p.advance()
		right := p.parseAssignment()
		node := p.newNode(KindAssignmentExpression, left, right)
		setField(node, "left", left)
		setField(node, "right", right)
		node.Tok = opTok
		return node
	}
	return left
}

func (p *parser) parseTernary() *Node {
	cond := p.parseBinary(bpLogicalOr)
	if !p.at(token.Question) {
		return cond
	}
	p.advance()
	savedSuppressTagCast := p.suppressTagCast
	if p.cur().Kind == token.Identifier && p.peek(1).Kind == token.Colon {
		p.suppressTagCast = true
	}
	consequence := p.parseAssignment()
	p.suppressTagCast = savedSuppressTagCast
	if !p.at(token.Colon) {
		node := p.newNode(KindTernaryExpression, cond, consequence)
		node.HasError = true
		return node
	}
	p.advance()
	alternative := p.parseAssignment()
	node := p.newNode(KindTernaryExpression, cond, consequence, alternative)
	setField(node, "condition", cond)
	setField(node, "consequence", consequence)
	setField(node, "alternative", alternative)
	return node
}

func (p *parser) parseBinary(minBP int) *Node {
	left := p.parseUnary()
	for {
		bp, ok := binaryBindingPower(p.cur().Kind)
		if !ok || bp < minBP {
			return left
		}
		opTok := p.advance()
		right := p.parseBinary(bp + 1)
		node := p.newNode(KindBinaryExpression, left, right)
		setField(node, "left", left)
		setField(node, "right", right)
		node.Tok = opTok
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

func (p *parser) parseUnary() *Node {
	if !p.enterDepth() {
		defer p.exitDepth()
		p.broken = true
		tok := p.cur()
		n := p.newLeaf(KindLiteral, tok)
		n.HasError = true
		return n
	}
	defer p.exitDepth()

	if isUnaryOp(p.cur().Kind) {
		opTok := p.advance()
		operand := p.parseUnary()
		node := p.newNode(KindUnaryExpression, operand)
		setField(node, "expression", operand)
		node.Tok = opTok
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

func isTagCastStart(p *parser) bool {
	if p.suppressTagCast {
		return false
	}
	if p.cur().Kind != token.Identifier {
		return false
	}
	if p.peek(1).Kind != token.Colon {
		return false
	}
	name := p.cur().Text(p.source)
	return p.knowsTag(name) || looksLikeTagName(name)
}

func looksLikeTagName(name string) bool {
	if name == "_" || name == "bool" {
		return true
	}
	if len(name) == 0 {
		return false
	}
	return name[0] >= 'A' && name[0] <= 'Z'
}

func (p *parser) parseTaggedExpression() *Node {
	tagTok := p.advance()
	tag := p.newLeaf(KindIdentifier, tagTok)
	p.advance()
	operand := p.parseUnary()
	node := p.newNode(KindTaggedExpression, tag, operand)
	setField(node, "tag", tag)
	setField(node, "expression", operand)
	return node
}

func (p *parser) parseSizeofLike(kind Kind) *Node {
	kwTok := p.advance()
	if p.at(token.LParen) {
		p.advance()
		inner := p.parseExpression()
		node := p.newNode(kind, inner)
		setField(node, "expression", inner)
		node.Tok = kwTok
		node.Start = kwTok.Start.Offset
		node.Leading = kwTok.LeadingTrivia
		if p.at(token.RParen) {
			rp := p.advance()
			node.End = rp.End.Offset
			node.Trailing = rp.TrailingTrivia
		} else {
			node.HasError = true
		}
		return node
	}

	if !p.at(token.Identifier) {
		leaf := p.newLeaf(KindIdentifier, kwTok)
		leaf.HasError = true
		return leaf
	}
	operand := p.parsePrimary()
	for p.at(token.LBracket) {
		operand = p.parseSubscript(operand)
	}
	node := p.newNode(kind, operand)
	setField(node, "expression", operand)
	node.Tok = kwTok
	node.Start = kwTok.Start.Offset
	node.Leading = kwTok.LeadingTrivia
	return node
}

func (p *parser) parseDefinedExpression() *Node {
	kwTok := p.advance()
	if !p.at(token.LParen) {
		if p.at(token.Identifier) {
			name := p.newLeaf(KindIdentifier, p.advance())
			node := p.newNode(KindDefinedExpression, name)
			setField(node, "name", name)
			node.Tok = kwTok
			return node
		}
		leaf := p.newLeaf(KindIdentifier, kwTok)
		leaf.HasError = true
		return leaf
	}
	p.advance()
	var name *Node
	if p.at(token.Identifier) {
		name = p.newLeaf(KindIdentifier, p.advance())
	}
	node := p.newNode(KindDefinedExpression, name)
	setField(node, "name", name)
	node.Tok = kwTok
	if p.at(token.RParen) {
		rp := p.advance()
		node.End = rp.End.Offset
		node.Trailing = rp.TrailingTrivia
	} else {
		node.HasError = true
	}
	return node
}
