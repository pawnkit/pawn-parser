package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser) parseVariableDeclaration() *Node {
	quals := p.collectQualifiers()
	return p.parseVariableDeclarationWithQualifiers(quals)
}

func (p *parser) parseVariableDeclarationWithQualifiers(quals []*Node) *Node {
	start := p.cur().Start.Offset
	leading := p.cur().LeadingTrivia
	if len(quals) > 0 {
		start = quals[0].Start
		leading = quals[0].Leading
	}

	node := &Node{Kind: KindVariableDeclaration, Start: start, Leading: leading}
	for _, q := range quals {
		node.addChild(q)
	}

	declarators := p.parseDeclaratorList()
	for _, d := range declarators {
		node.addChild(d)
	}
	setField(node, "storage", firstOrNil(quals))

	if p.at(token.Semicolon) {
		semi := p.advance()
		node.End = semi.End.Offset
		node.Trailing = semi.TrailingTrivia
	} else {
		node.HasError = true
	}
	return node
}

func (p *parser) parseDeclaratorList() []*Node {
	return p.parseItemSequence(itemGrammar{
		parseItem: parseCommaListItem((*parser).parseDeclarator),
		stop:      func(p *parser) bool { return p.at(token.Semicolon) },
	})
}

func (p *parser) parseDeclarator() *Node {
	start := p.cur().Start.Offset
	leading := p.cur().LeadingTrivia
	node := &Node{Kind: KindVariableDeclarator, Start: start, Leading: leading}

	tag := p.parseOptionalTagPrefix()
	if tag != nil {
		setField(node, "tag", tag)
		node.addChild(tag)
	}

	if !p.at(token.Identifier) {
		node.HasError = true
		return node
	}
	nameTok := p.advance()
	name := p.newLeaf(KindIdentifier, nameTok)
	setField(node, "name", name)
	node.addChild(name)
	node.End = name.End
	node.Trailing = name.Trailing

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
		init := p.parseDeclaratorInitializer()
		setField(node, "initializer", init)
		node.addChild(init)
		node.End = init.End
		node.Trailing = init.Trailing
	}
	return node
}

func (p *parser) parseDeclaratorInitializer() *Node {
	if p.at(token.LBrace) {
		return p.parseArrayLiteral()
	}
	return p.parseAssignment()
}

func (p *parser) parseEnumDeclaration(quals []*Node) *Node {
	start := p.cur().Start.Offset
	leading := p.cur().LeadingTrivia
	if len(quals) > 0 {
		start = quals[0].Start
		leading = quals[0].Leading
	}
	kw := p.advance()
	node := &Node{Kind: KindEnumDeclaration, Tok: kw, Start: start, Leading: leading}
	for _, q := range quals {
		node.addChild(q)
	}

	if p.at(token.Identifier) {
		name := p.newLeaf(KindIdentifier, p.advance())
		setField(node, "name", name)
		node.addChild(name)
	}

	if increment := p.parseEnumIncrementClause(); increment != nil {
		setField(node, "increment", increment)
		node.addChild(increment)
	}

	tag := p.parseOptionalTagPrefix()
	if tag != nil {
		setField(node, "tag", tag)
		node.addChild(tag)
	}

	if !p.at(token.LBrace) {
		node.HasError = true
		return node
	}
	lb := p.advance()
	body := &Node{Kind: KindBlock, Start: lb.Start.Offset, Leading: lb.LeadingTrivia}
	items := p.parseItemSequence(itemGrammar{
		parseItem: parseCommaListItem((*parser).parseEnumEntry),
		stop: func(p *parser) bool {
			p.abortIfSharedAcrossBranch()
			return p.at(token.RBrace)
		},
	})
	for _, it := range items {
		body.addChild(it)
	}
	if p.at(token.RBrace) {
		rb := p.advance()
		body.End = rb.End.Offset
		body.Trailing = rb.TrailingTrivia
	} else {
		body.HasError = true
	}
	setField(node, "body", body)
	node.addChild(body)
	node.End = body.End
	node.Trailing = body.Trailing

	if p.at(token.Semicolon) {
		semi := p.advance()
		node.End = semi.End.Offset
		node.Trailing = semi.TrailingTrivia
	}
	return node
}

func (p *parser) parseEnumIncrementClause() *Node {
	if !p.at(token.LParen) {
		return nil
	}
	lp := p.advance()
	depth := 1
	var last token.Token
	for !p.atEnd() && depth > 0 {
		last = p.advance()
		switch last.Kind {
		case token.LParen:
			depth++
		case token.RParen:
			depth--
		default:
			// Other tokens don't affect paren depth.
		}
	}
	n := rawNode(p.source, lp.Start.Offset, last.End.Offset)
	n.HasError = false
	n.Leading = lp.LeadingTrivia
	n.Trailing = last.TrailingTrivia
	return n
}

func (p *parser) parseEnumEntry() *Node {
	start := p.cur().Start.Offset
	leading := p.cur().LeadingTrivia
	node := &Node{Kind: KindEnumEntry, Start: start, Leading: leading}

	tag := p.parseOptionalTagPrefix()
	if tag != nil {
		setField(node, "tag", tag)
		node.addChild(tag)
	}

	if !p.at(token.Identifier) {
		node.HasError = true
		if !p.atEnd() && p.cur().Kind != token.Comma && p.cur().Kind != token.RBrace {
			bad := p.advance()
			node.End = bad.End.Offset
			node.Trailing = bad.TrailingTrivia
		}
		return node
	}
	name := p.newLeaf(KindIdentifier, p.advance())
	setField(node, "name", name)
	node.addChild(name)
	node.End = name.End
	node.Trailing = name.Trailing

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
		val := p.parseTernary()
		setField(node, "value", val)
		node.addChild(val)
		node.End = val.End
		node.Trailing = val.Trailing
	}
	return node
}
