package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser) parseParameterList() *Node {
	if !p.at(token.LParen) {
		n := &Node{Kind: KindParameterList, HasError: true}
		return n
	}
	lp := p.advance()
	node := &Node{Kind: KindParameterList, Start: lp.Start.Offset, Leading: lp.LeadingTrivia}
	items := p.parseItemSequence(itemGrammar{
		parseItem: parseCommaListItem((*parser).parseParameter),
		stop: func(p *parser) bool {
			p.abortIfSharedAcrossBranch()
			return p.at(token.RParen)
		},
	})
	for _, it := range items {
		node.addChild(it)
	}
	if p.at(token.RParen) {
		rp := p.advance()
		node.End = rp.End.Offset
		node.Trailing = rp.TrailingTrivia
	} else {
		node.HasError = true
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
	node := &Node{Kind: KindParameter, Start: start, Leading: leading}
	p.parseParameterQualifiers(node)

	if p.at(token.Amp) {
		p.advance()
	}

	tag := p.parseOptionalTagPrefix()
	if tag != nil {
		setField(node, "tag", tag)
		node.addChild(tag)
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
		node.addChild(p.newLeaf(KindIdentifier, p.advance()))
	}
}

func (p *parser) parseParameterName(node *Node) bool {
	if !p.at(token.Identifier) {
		node.HasError = true
		if !p.atEnd() && p.cur().Kind != token.Comma && p.cur().Kind != token.RParen {
			bad := p.advance()
			node.End = bad.End.Offset
			node.Trailing = bad.TrailingTrivia
		}
		return false
	}
	nameTok := p.advance()
	name := p.newLeaf(KindIdentifier, nameTok)
	setField(node, "name", name)
	node.addChild(name)
	node.End = name.End
	node.Trailing = name.Trailing
	return true
}

func (p *parser) parseParameterSuffix(node *Node) {
	dims := p.parseDimensions()
	for _, d := range dims {
		node.addChild(d)
		node.End = d.End
		node.Trailing = d.Trailing
	}
	if len(dims) > 0 {
		setField(node, "array", dims[0])
	}

	if p.at(token.Assign) {
		p.advance()
		def := p.parseAssignment()
		setField(node, "default_value", def)
		node.addChild(def)
		node.End = def.End
		node.Trailing = def.Trailing
	}
}
