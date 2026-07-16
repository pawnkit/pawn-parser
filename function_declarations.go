package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser[N, S]) canStartUnbracedFunctionBody() bool {
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

func (p *parser[N, S]) parseUnbracedFunctionBody() N {
	if p.at(token.Hash) && p.peekDirectiveKeyword() == dirIf {
		startPos := p.pos
		mark := p.sink.Mark()
		region, ok := p.trySingleStatementConditional()
		if ok {
			return region
		}
		p.pos = startPos
		p.sink.Rewind(mark)
		return p.rawConditionalRegion()
	}
	return p.parseStatement()
}

func (p *parser[N, S]) trySingleStatementConditional() (node N, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, isAbort := r.(condAbort); isAbort {
				node, ok = p.sink.Nil(), false
				return
			}
			panic(r)
		}
	}()

	p.condDepth++
	defer func() { p.condDepth-- }()

	region := p.sink.Store(Node{Kind: KindConditionalRegion, Start: p.cur().Start.Offset, Leading: p.cur().LeadingTrivia})
	for {
		if !p.at(token.Hash) {
			return p.sink.Nil(), false
		}
		dk := p.peekDirectiveKeyword()
		directive := p.consumeRawDirectiveLine(p.cur().Start.Offset, directiveNodeKind(dk))
		branch := p.sink.Store(Node{Kind: KindConditionalBranch, Start: p.sink.Start(directive), End: p.sink.End(directive), Leading: p.sink.Leading(directive), Trailing: p.sink.Trailing(directive)})
		p.sink.SetField(branch, fieldDirective, directive)
		p.sink.AddChild(branch, directive)
		p.sink.AddChild(region, branch)

		if dk == dirEndif {
			break
		}
		if dk != dirIf && dk != dirElseif && dk != dirElse {
			return p.sink.Nil(), false
		}

		stmt := p.parseStatement()
		p.sink.AddChild(branch, stmt)
		p.sink.SetEnd(branch, p.sink.End(stmt))
		p.sink.SetTrailing(branch, p.sink.Trailing(stmt))
	}
	p.sink.SetEnd(region, p.sink.End(p.sink.Children(region)[len(p.sink.Children(region))-1]))
	return region, true
}

func (p *parser[N, S]) parseFunctionLike(quals []N) N {
	start := 0
	var leading []token.Trivia
	if len(quals) > 0 {
		start = p.sink.Start(quals[0])
		leading = p.sink.Leading(quals[0])
	} else {
		start = p.cur().Start.Offset
		leading = p.cur().LeadingTrivia
	}

	tag := p.parseOptionalTagPrefix()
	callingConvention := p.parseDimensions()
	name := p.parseFunctionName()
	nameDimensions := p.parseDimensions()
	var generic N
	if p.at(token.Lt) {
		generic = p.parseStateSelector()
	}

	params := p.parseParameterList()

	stateSel := p.parseStateSelector()

	kind := KindFunctionDeclaration
	var body N
	switch {
	case p.at(token.LBrace):
		kind = KindFunctionDefinition
		body = p.parseBlock()
	case p.canStartUnbracedFunctionBody():
		kind = KindFunctionDefinition
		body = p.parseUnbracedFunctionBody()
	}

	node := p.sink.Store(Node{Kind: kind, Start: start, Leading: leading})
	for _, q := range quals {
		p.sink.AddChild(node, q)
	}
	p.sink.SetField(node, fieldStorage, p.firstOrNil(quals))
	if tag != p.sink.Nil() {
		p.sink.SetField(node, fieldTag, tag)
		p.sink.AddChild(node, tag)
	}
	for _, dimension := range callingConvention {
		p.sink.AddChild(node, dimension)
	}
	p.sink.SetField(node, fieldCallingConvention, p.firstOrNil(callingConvention))
	p.sink.SetField(node, fieldName, name)
	p.sink.AddChild(node, name)
	for _, dimension := range nameDimensions {
		p.sink.AddChild(node, dimension)
	}
	p.sink.SetField(node, fieldDimensions, p.firstOrNil(nameDimensions))
	if generic != p.sink.Nil() {
		p.sink.SetField(node, fieldGeneric, generic)
		p.sink.AddChild(node, generic)
	}
	p.sink.SetField(node, fieldParameters, params)
	p.sink.AddChild(node, params)
	if stateSel != p.sink.Nil() {
		p.sink.SetField(node, fieldState, stateSel)
		p.sink.AddChild(node, stateSel)
	}

	if body != p.sink.Nil() {
		p.sink.SetField(node, fieldBody, body)
		p.sink.AddChild(node, body)
		return node
	}

	if p.at(token.Assign) {
		p.advance()
		alias := p.parseAssignment()
		p.sink.SetField(node, fieldAlias, alias)
		p.sink.AddChild(node, alias)
		p.sink.SetEnd(node, p.sink.End(alias))
		p.sink.SetTrailing(node, p.sink.Trailing(alias))
	}

	if p.at(token.Semicolon) {
		semi := p.advance()
		p.sink.SetEnd(node, semi.End.Offset)
		p.sink.SetTrailing(node, semi.TrailingTrivia)
	} else {
		p.sink.SetHasError(node, true)
	}
	return node
}

func (p *parser[N, S]) firstOrNil(nodes []N) N {
	if len(nodes) == 0 {
		return p.sink.Nil()
	}
	return nodes[0]
}
