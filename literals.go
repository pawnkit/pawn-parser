package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser) parsePrimary() *Node {
	tok := p.cur()
	if isKeywordToken(tok.Kind) && p.peek(1).Kind == token.LParen {
		p.advance()
		return p.newLeaf(KindIdentifier, tok)
	}
	switch tok.Kind {
	case token.Identifier, token.MacroParam:
		p.advance()
		return p.newLeaf(KindIdentifier, tok)
	case token.IntLiteral, token.FloatLiteral, token.CharLiteral, token.KwNull:
		p.advance()
		return p.newLeaf(KindLiteral, tok)
	case token.StringLiteral, token.PackedString:
		return p.parseStringConcat()
	case token.Hash:
		if p.peek(1).Kind == token.Identifier {
			return p.parseStringConcat()
		}
		p.advance()
		n := p.newLeaf(KindLiteral, tok)
		n.HasError = true
		return n
	case token.Ellipsis:
		p.advance()
		return p.newLeaf(KindLiteral, tok)
	case token.LParen:
		return p.parseParenthesized()
	case token.LBrace:
		return p.parseArrayLiteral()
	default:
		p.advance()
		n := p.newLeaf(KindLiteral, tok)
		n.HasError = true
		return n
	}
}

func (p *parser) isStringPartStart() bool {
	if p.at(token.StringLiteral) || p.at(token.PackedString) {
		return true
	}
	return p.at(token.Hash) && p.peek(1).Kind == token.Identifier
}

func (p *parser) parseStringPart() *Node {
	if p.at(token.Hash) {
		return p.parseStringizeExpression()
	}
	tok := p.advance()
	return p.newLeaf(KindLiteral, tok)
}

func (p *parser) parseStringizeExpression() *Node {
	hash := p.advance() // '#'
	nameTok := p.advance()
	name := p.newLeaf(KindIdentifier, nameTok)
	node := &Node{Kind: KindStringizeExpression, Tok: hash, Start: hash.Start.Offset, End: nameTok.End.Offset, Leading: hash.LeadingTrivia, Trailing: nameTok.TrailingTrivia}
	setField(node, "name", name)
	node.addChild(name)
	return node
}

func (p *parser) parseStringConcat() *Node {
	first := p.parseStringPart()
	if !p.isStringPartStart() {
		return first
	}
	concat := &Node{Kind: KindStringConcat, Start: first.Start, Leading: first.Leading}
	concat.addChild(first)
	for p.isStringPartStart() {
		concat.addChild(p.parseStringPart())
	}
	return concat
}

func (p *parser) parseParenthesized() *Node {
	lp := p.advance()
	inner := p.parseExpression()
	node := p.newNode(KindParenthesizedExpression, inner)
	setField(node, "expression", inner)
	node.Start = lp.Start.Offset
	node.Leading = lp.LeadingTrivia
	if p.at(token.RParen) {
		rp := p.advance()
		node.End = rp.End.Offset
		node.Trailing = rp.TrailingTrivia
	} else {
		node.HasError = true
	}
	return node
}

func (p *parser) parseArrayLiteral() *Node {
	lb := p.advance() // '{'
	return p.parseBracketedList(KindArrayLiteral, lb, token.RBrace, (*parser).parseAssignment)
}
