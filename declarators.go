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

	node := p.storeNode(Node{Kind: KindVariableDeclaration, Start: start, Leading: leading})
	for _, q := range quals {
		p.addChild(node, q)
	}

	declarators := p.parseDeclaratorList()
	for _, d := range declarators {
		p.addChild(node, d)
	}
	p.setField(node, "storage", firstOrNil(quals))

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
		parseItem:                 parseCommaListItem((*parser).parseDeclarator),
		stop:                      func(p *parser) bool { return p.at(token.Semicolon) },
		preserveRecoverySemicolon: true,
		recoveryContext:           "declarator",
		recoveryExpected:          []token.Kind{token.Comma, token.Semicolon},
	})
}

func (p *parser) parseDeclarator() *Node {
	start := p.cur().Start.Offset
	leading := p.cur().LeadingTrivia
	node := p.storeNode(Node{Kind: KindVariableDeclarator, Start: start, Leading: leading})

	tag := p.parseOptionalTagPrefix()
	if tag != nil {
		p.setField(node, "tag", tag)
		p.addChild(node, tag)
	}

	if !isFunctionNameToken(p.cur().Kind) {
		p.emitMissing(DiagnosticMissingIdentifier, "expected declarator name", token.Identifier)
		node.HasError = true
		return node
	}
	name := p.parseQualifiedIdentifier()
	p.setField(node, "name", name)
	p.addChild(node, name)
	node.End = name.End
	node.Trailing = name.Trailing
	if p.at(token.Lt) {
		selector := p.parseStateSelector()
		p.setField(node, "capacity", selector)
		p.addChild(node, selector)
	}

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
		selector := p.parseStateSelector()
		p.setField(node, "capacity", selector)
		p.addChild(node, selector)
	}

	if p.at(token.Assign) {
		p.advance()
		init := p.parseDeclaratorInitializer()
		p.setField(node, "initializer", init)
		p.addChild(node, init)
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
	node := p.storeNode(Node{Kind: KindEnumDeclaration, Tok: kw, Start: start, Leading: leading})
	for _, q := range quals {
		p.addChild(node, q)
	}

	if p.at(token.Identifier) {
		name := p.parseQualifiedIdentifier()
		p.setField(node, "name", name)
		p.addChild(node, name)
	}

	var tag *Node
	switch {
	case p.at(token.Colon) && p.peek(1).Kind == token.LBrace:
		colon := p.advance()
		tag = p.directiveSpan(KindTaggedType, colon.Start.Offset, colon.End.Offset, colon.LeadingTrivia, colon.TrailingTrivia)
	case p.at(token.Colon) && p.peek(1).Kind == token.Identifier:
		colon := p.advance()
		tag = p.newLeaf(KindTaggedType, p.advance())
		tag.Start = colon.Start.Offset
	default:
		tag = p.parseOptionalTagPrefix()
	}
	if tag != nil {
		p.setField(node, "tag", tag)
		p.addChild(node, tag)
	}

	if increment := p.parseEnumIncrementClause(); increment != nil {
		p.setField(node, "increment", increment)
		p.addChild(node, increment)
	}

	if !p.at(token.LBrace) {
		p.emitMissing(DiagnosticMissingDeclaration, "expected enum body", token.LBrace)
		node.HasError = true
		return node
	}
	lb := p.advance()
	body := p.storeNode(Node{Kind: KindBlock, Start: lb.Start.Offset, Leading: lb.LeadingTrivia})
	items := p.parseItemSequence(itemGrammar{
		parseItem: parseCommaListItem((*parser).parseEnumEntry),
		stop: func(p *parser) bool {
			p.abortIfSharedAcrossBranch()
			return p.at(token.RBrace)
		},
	})
	for _, it := range items {
		p.addChild(body, it)
	}
	if p.at(token.RBrace) {
		rb := p.advance()
		body.End = rb.End.Offset
		body.Trailing = rb.TrailingTrivia
	} else {
		body.HasError = true
		p.emitMissingToken(token.RBrace, "enum body")
	}
	p.setField(node, "body", body)
	p.addChild(node, body)
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
	n := p.directiveSpan(KindEnumIncrementClause, lp.Start.Offset, last.End.Offset, lp.LeadingTrivia, last.TrailingTrivia)
	n.Leading = lp.LeadingTrivia
	n.Trailing = last.TrailingTrivia
	return n
}

func (p *parser) parseEnumEntry() *Node {
	start := p.cur().Start.Offset
	leading := p.cur().LeadingTrivia
	node := p.storeNode(Node{Kind: KindEnumEntry, Start: start, Leading: leading})

	tag := p.parseOptionalTagPrefix()
	if tag != nil {
		p.setField(node, "tag", tag)
		p.addChild(node, tag)
	}

	if !p.at(token.Identifier) {
		p.emitMissing(DiagnosticMissingIdentifier, "expected enum entry name", token.Identifier)
		node.HasError = true
		if !p.atEnd() && p.cur().Kind != token.Comma && p.cur().Kind != token.RBrace {
			bad := p.advance()
			node.End = bad.End.Offset
			node.Trailing = bad.TrailingTrivia
		}
		return node
	}
	name := p.parseQualifiedIdentifier()
	p.setField(node, "name", name)
	p.addChild(node, name)
	node.End = name.End
	node.Trailing = name.Trailing

	dims := p.parseDimensions()
	for _, d := range dims {
		p.addChild(node, d)
		node.End = d.End
		node.Trailing = d.Trailing
	}
	if len(dims) > 0 {
		p.setField(node, "array", dims[0])
	}

	if p.at(token.Assign) {
		p.advance()
		val := p.parseTernary()
		p.setField(node, "value", val)
		p.addChild(node, val)
		node.End = val.End
		node.Trailing = val.Trailing
	}
	return node
}
