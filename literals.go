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
		if isExpressionBoundary(tok.Kind) {
			p.emitMissing(DiagnosticMissingExpression, "expected expression",
				token.Identifier, token.IntLiteral, token.LParen)
			n := &Node{
				Kind: KindLiteral, Tok: tok, Start: tok.Start.Offset, End: tok.Start.Offset, HasError: true,
				Leading: tok.LeadingTrivia,
			}
			return n
		}
		p.advance()
		n := p.newLeaf(KindLiteral, tok)
		n.HasError = true
		p.emitDiagnostic(Diagnostic{
			Code:    DiagnosticUnexpectedToken,
			Message: "unexpected token in expression", Range: tokenRange(tok), Found: tok,
			Recovery: Recovery{Kind: RecoveryRemove, Range: tokenRange(tok), Confidence: RecoveryExact},
		})
		return n
	}
}

func isExpressionBoundary(kind token.Kind) bool {
	switch kind {
	case token.EOF, token.RParen, token.RBracket, token.RBrace, token.Comma, token.Semicolon:
		return true
	default:
		return false
	}
}

func (p *parser) isStringPartStart() bool {
	if p.at(token.StringLiteral) || p.at(token.PackedString) {
		return true
	}
	if p.at(token.Identifier) || p.at(token.MacroParam) || isKeywordToken(p.cur().Kind) {
		return true
	}
	return p.at(token.Hash) && p.peek(1).Kind == token.Identifier
}

func (p *parser) parseStringPart() *Node {
	if p.at(token.Hash) {
		return p.parseStringizeExpression()
	}
	tok := p.advance()
	if tok.Kind == token.Identifier || tok.Kind == token.MacroParam || isKeywordToken(tok.Kind) {
		return p.newLeaf(KindIdentifier, tok)
	}
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
		p.emitMissingToken(token.RParen, "parenthesized expression")
	}
	return node
}

func (p *parser) parseArrayLiteral() *Node {
	lb := p.advance() // '{'
	return p.parseBracketedList(KindArrayLiteral, lb, token.RBrace, (*parser).parseAssignment)
}
