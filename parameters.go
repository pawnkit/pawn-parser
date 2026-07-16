package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser) parseParameterList() *Node {
	if !p.at(token.LParen) {
		p.emitMissingToken(token.LParen, "parameter list")
		n := p.storeNode(Node{Kind: KindParameterList, HasError: true})
		return n
	}
	lp := p.advance()
	node := p.storeNode(Node{Kind: KindParameterList, Start: lp.Start.Offset, Leading: lp.LeadingTrivia})
	items := p.parseItemSequence(itemGrammar{
		parseItem:      (*parser).parseParameter,
		stopKind:       token.RParen,
		abortAtStop:    true,
		commaSeparated: true,
	})
	for _, it := range items {
		p.addChild(node, it)
	}
	if p.at(token.RParen) {
		rp := p.advance()
		node.End = rp.End.Offset
		node.Trailing = rp.TrailingTrivia
	} else {
		node.HasError = true
		p.emitMissingToken(token.RParen, "parameter list")
	}
	return node
}

func (p *parser) parseParameter() *Node {
	if p.at(token.Ellipsis) {
		tok := p.advance()
		return p.newLeaf(KindParameter, tok)
	}

	start := p.cur().Start.Offset
	leading := p.cur().LeadingTrivia
	node := p.storeNode(Node{Kind: KindParameter, Start: start, Leading: leading})
	p.parseParameterQualifiers(node)

	if p.at(token.Amp) {
		p.advance()
	}

	tag := p.parseOptionalTagPrefix()
	if tag != nil {
		p.setField(node, "tag", tag)
		p.addChild(node, tag)
	}

	if p.at(token.Amp) {
		p.advance()
	}

	if p.at(token.Ellipsis) {
		tok := p.advance()
		node.End = tok.End.Offset
		node.Trailing = tok.TrailingTrivia
		return node
	}

	if !p.parseParameterName(node) {
		return node
	}
	p.parseParameterSuffix(node)
	return node
}

func (p *parser) parseParameterQualifiers(node *Node) {
	for p.at(token.KwConst) || p.at(token.KwStock) {
		p.addChild(node, p.newLeaf(KindIdentifier, p.advance()))
	}
}

func (p *parser) parseParameterName(node *Node) bool {
	if !isFunctionNameToken(p.curKind()) {
		p.emitMissing(DiagnosticMissingIdentifier, "expected parameter name", token.Identifier)
		node.HasError = true
		if !p.atEnd() && p.curKind() != token.Comma && p.curKind() != token.RParen {
			bad := p.advance()
			node.End = bad.End.Offset
			node.Trailing = bad.TrailingTrivia
		}
		return false
	}
	name := p.parseQualifiedIdentifier()
	p.setField(node, "name", name)
	p.addChild(node, name)
	node.End = name.End
	node.Trailing = name.Trailing
	return true
}

func (p *parser) parseParameterSuffix(node *Node) {
	dims := p.parseDimensions()
	for _, d := range dims {
		p.addChild(node, d)
		node.End = d.End
		node.Trailing = d.Trailing
	}
	if len(dims) > 0 {
		p.setField(node, "array", dims[0])
	}
	if p.at(token.Lt) {
		generic := p.parseStateSelector()
		p.setField(node, "generic", generic)
		p.addChild(node, generic)
	}

	if p.at(token.Assign) {
		p.advance()
		def := p.parseAssignment()
		p.setField(node, "default_value", def)
		p.addChild(node, def)
		node.End = def.End
		node.Trailing = def.Trailing
	}
}
