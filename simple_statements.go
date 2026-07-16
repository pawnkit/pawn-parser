package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser) parseReturnStatement() *Node {
	kw := p.advance()
	node := p.storeNode(Node{Kind: KindReturnStatement, Tok: kw, Start: kw.Start.Offset, Leading: kw.LeadingTrivia})
	if !p.at(token.Semicolon) {
		val := p.parseExpression()
		p.setField(node, "value", val)
		p.addChild(node, val)
	}
	p.consumeTrailingSemi(node)
	return node
}

func (p *parser) simpleKeywordStatement(kind Kind) *Node {
	kw := p.advance()
	node := p.storeNode(Node{Kind: kind, Tok: kw, Start: kw.Start.Offset, Leading: kw.LeadingTrivia})
	if p.at(token.Semicolon) {
		semi := p.advance()
		node.End = semi.End.Offset
		node.Trailing = semi.TrailingTrivia
	} else {
		node.End = kw.End.Offset
		node.Trailing = kw.TrailingTrivia
		if p.missingSemiOK() {
			node.MissingSemi = true
		} else {
			node.HasError = true
		}
	}
	return node
}

func (p *parser) parseGotoStatement() *Node {
	kw := p.advance()
	node := p.storeNode(Node{Kind: KindGotoStatement, Tok: kw, Start: kw.Start.Offset, Leading: kw.LeadingTrivia})
	if p.at(token.Identifier) {
		label := p.newLeaf(KindIdentifier, p.advance())
		p.setField(node, "label", label)
		p.addChild(node, label)
	} else {
		node.HasError = true
	}
	p.consumeTrailingSemi(node)
	return node
}

func (p *parser) parseLabelStatement() *Node {
	nameTok := p.advance()
	name := p.newLeaf(KindIdentifier, nameTok)
	colon := p.advance() // ':'
	node := p.storeNode(Node{Kind: KindLabelStatement, Start: nameTok.Start.Offset, Leading: nameTok.LeadingTrivia})
	p.setField(node, "label", name)
	p.addChild(node, name)
	node.End = colon.End.Offset
	node.Trailing = colon.TrailingTrivia
	return node
}

func (p *parser) parseStateStatement() *Node {
	kw := p.advance()
	node := p.storeNode(Node{Kind: KindStateStatement, Tok: kw, Start: kw.Start.Offset, Leading: kw.LeadingTrivia})
	if !p.at(token.Identifier) {
		node.HasError = true
		p.consumeTrailingSemi(node)
		return node
	}

	name := p.newLeaf(KindIdentifier, p.advance())
	p.setField(node, "state", name)
	p.addChild(node, name)
	p.parseStateStatementTarget(node)
	p.consumeTrailingSemi(node)
	return node
}

func (p *parser) parseStateStatementTarget(node *Node) {
	if !p.at(token.Colon) {
		return
	}
	p.advance()
	if !p.at(token.Identifier) && !isKeywordToken(p.cur().Kind) {
		node.HasError = true
		return
	}
	target := p.newLeaf(KindIdentifier, p.advance())
	p.setField(node, "target", target)
	p.addChild(node, target)
}

func (p *parser) parseExpressionStatement() *Node {
	expr := p.parseExpression()
	node := p.storeNode(Node{Kind: KindExpressionStatement, Start: expr.Start, Leading: expr.Leading})
	p.setField(node, "expression", expr)
	p.addChild(node, expr)
	if p.at(token.Hash) && iteratorCallExpression(expr) {
		switch p.peekDirectiveKeyword() {
		case dirElseif, dirElse, dirEndif:
			node.MissingSemi = true
			return node
		}
	}
	p.consumeTrailingSemi(node)
	return node
}

func iteratorCallExpression(expr *Node) bool {
	if expr == nil || expr.Kind != KindCallExpression {
		return false
	}
	args := expr.Field("arguments")
	if args == nil {
		return false
	}
	for _, child := range args.Children {
		if child.Kind == KindIteratorArgument {
			return true
		}
	}
	return false
}
