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

	region := p.storeNode(Node{Kind: KindConditionalRegion, Start: p.cur().Start.Offset, Leading: p.cur().LeadingTrivia})
	for {
		if !p.at(token.Hash) {
			return nil, false
		}
		dk := p.peekDirectiveKeyword()
		directive := p.consumeRawDirectiveLine(p.cur().Start.Offset, directiveNodeKind(dk))
		branch := p.storeNode(Node{Kind: KindConditionalBranch, Start: directive.Start, End: directive.End, Leading: directive.Leading, Trailing: directive.Trailing})
		p.setField(branch, fieldDirective, directive)
		p.addChild(branch, directive)
		p.addChild(region, branch)

		if dk == dirEndif {
			break
		}
		if dk != dirIf && dk != dirElseif && dk != dirElse {
			return nil, false
		}

		stmt := p.parseStatement()
		p.addChild(branch, stmt)
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
	nameDimensions := p.parseDimensions()
	var generic *Node
	if p.at(token.Lt) {
		generic = p.parseStateSelector()
	}

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

	node := p.storeNode(Node{Kind: kind, Start: start, Leading: leading})
	for _, q := range quals {
		p.addChild(node, q)
	}
	p.setField(node, fieldStorage, firstOrNil(quals))
	if tag != nil {
		p.setField(node, fieldTag, tag)
		p.addChild(node, tag)
	}
	for _, dimension := range callingConvention {
		p.addChild(node, dimension)
	}
	p.setField(node, fieldCallingConvention, firstOrNil(callingConvention))
	p.setField(node, fieldName, name)
	p.addChild(node, name)
	for _, dimension := range nameDimensions {
		p.addChild(node, dimension)
	}
	p.setField(node, fieldDimensions, firstOrNil(nameDimensions))
	if generic != nil {
		p.setField(node, fieldGeneric, generic)
		p.addChild(node, generic)
	}
	p.setField(node, fieldParameters, params)
	p.addChild(node, params)
	if stateSel != nil {
		p.setField(node, fieldState, stateSel)
		p.addChild(node, stateSel)
	}

	if body != nil {
		p.setField(node, fieldBody, body)
		p.addChild(node, body)
		return node
	}

	if p.at(token.Assign) {
		p.advance()
		alias := p.parseAssignment()
		p.setField(node, fieldAlias, alias)
		p.addChild(node, alias)
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
