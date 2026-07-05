package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser) canStartUnbracedFunctionBody() bool {
	if p.atEnd() || p.at(token.Semicolon) || p.at(token.Assign) {
		return false
	}
	if p.at(token.Hash) {
		switch p.peekDirectiveKeyword() {
		case dirElseif, dirElse, dirEndif:
			return false
		}
	}
	return true
}

func (p *parser) parseUnbracedFunctionBody() *Node {
	if p.at(token.Hash) && p.peekDirectiveKeyword() == dirIf {
		startPos := p.pos
		region, ok := p.trySingleStatementConditional()
		if ok {
			return region
		}
		p.pos = startPos
		return p.rawConditionalRegion()
	}
	return p.parseStatement()
}

func (p *parser) trySingleStatementConditional() (node *Node, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, isAbort := r.(condAbort); isAbort {
				node, ok = nil, false
				return
			}
			panic(r)
		}
	}()

	p.condDepth++
	defer func() { p.condDepth-- }()

	region := &Node{Kind: KindConditionalRegion, Start: p.cur().Start.Offset, Leading: p.cur().LeadingTrivia}
	for {
		if !p.at(token.Hash) {
			return nil, false
		}
		dk := p.peekDirectiveKeyword()
		directive := p.consumeRawDirectiveLine(p.cur().Start.Offset, directiveNodeKind(dk))
		branch := &Node{Kind: KindConditionalBranch, Start: directive.Start, End: directive.End, Leading: directive.Leading, Trailing: directive.Trailing}
		setField(branch, "directive", directive)
		branch.addChild(directive)
		region.addChild(branch)

		if dk == dirEndif {
			break
		}
		if dk != dirIf && dk != dirElseif && dk != dirElse {
			return nil, false
		}

		stmt := p.parseStatement()
		branch.addChild(stmt)
		branch.End = stmt.End
		branch.Trailing = stmt.Trailing
	}
	region.End = region.Children[len(region.Children)-1].End
	return region, true
}

func (p *parser) parseFunctionLike(quals []*Node) *Node {
	start := 0
	var leading []token.Trivia
	if len(quals) > 0 {
		start = quals[0].Start
		leading = quals[0].Leading
	} else {
		start = p.cur().Start.Offset
		leading = p.cur().LeadingTrivia
	}

	tag := p.parseOptionalTagPrefix()
	callingConvention := p.parseDimensions()
	name := p.parseFunctionName()

	params := p.parseParameterList()

	stateSel := p.parseStateSelector()

	kind := KindFunctionDeclaration
	var body *Node
	switch {
	case p.at(token.LBrace):
		kind = KindFunctionDefinition
		body = p.parseBlock()
	case p.canStartUnbracedFunctionBody():
		kind = KindFunctionDefinition
		body = p.parseUnbracedFunctionBody()
	}

	node := &Node{Kind: kind, Start: start, Leading: leading}
	for _, q := range quals {
		node.addChild(q)
	}
	setField(node, "storage", firstOrNil(quals))
	if tag != nil {
		setField(node, "tag", tag)
		node.addChild(tag)
	}
	for _, dimension := range callingConvention {
		node.addChild(dimension)
	}
	setField(node, "calling_convention", firstOrNil(callingConvention))
	setField(node, "name", name)
	node.addChild(name)
	setField(node, "parameters", params)
	node.addChild(params)
	if stateSel != nil {
		setField(node, "state", stateSel)
		node.addChild(stateSel)
	}

	if body != nil {
		setField(node, "body", body)
		node.addChild(body)
		return node
	}

	if p.at(token.Assign) {
		p.advance()
		alias := p.parseAssignment()
		setField(node, "alias", alias)
		node.addChild(alias)
		node.End = alias.End
		node.Trailing = alias.Trailing
	}

	if p.at(token.Semicolon) {
		semi := p.advance()
		node.End = semi.End.Offset
		node.Trailing = semi.TrailingTrivia
	} else {
		node.HasError = true
	}
	return node
}

func firstOrNil(nodes []*Node) *Node {
	if len(nodes) == 0 {
		return nil
	}
	return nodes[0]
}
