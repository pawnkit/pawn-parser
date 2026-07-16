package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser[N, S]) parseVariableDeclaration() N {
	quals := p.collectQualifiers()
	return p.parseVariableDeclarationWithQualifiers(quals)
}

func (p *parser[N, S]) parseVariableDeclarationWithQualifiers(quals []N) N {
	start := p.cur().Start.Offset
	leading := p.cur().LeadingTrivia
	if len(quals) > 0 {
		start = p.sink.Start(quals[0])
		leading = p.sink.Leading(quals[0])
	}

	node := p.sink.Store(Node{Kind: KindVariableDeclaration, Start: start, Leading: leading})
	for _, q := range quals {
		p.sink.AddChild(node, q)
	}

	declarators := p.parseDeclaratorList()
	for _, d := range declarators {
		p.sink.AddChild(node, d)
	}
	p.sink.SetField(node, fieldStorage, p.firstOrNil(quals))

	if p.at(token.Semicolon) {
		semi := p.advance()
		p.sink.SetEnd(node, semi.End.Offset)
		p.sink.SetTrailing(node, semi.TrailingTrivia)
	} else {
		p.sink.SetHasError(node, true)
	}
	return node
}

func (p *parser[N, S]) parseDeclaratorList() []N {
	return p.parseItemSequence(itemGrammar[N, S]{
		parseItem:                 (*parser[N, S]).parseDeclarator,
		stopKind:                  token.Semicolon,
		commaSeparated:            true,
		preserveRecoverySemicolon: true,
		recoveryContext:           "declarator",
		recoveryExpected:          []token.Kind{token.Comma, token.Semicolon},
	})
}

func (p *parser[N, S]) parseDeclarator() N {
	start := p.cur().Start.Offset
	leading := p.cur().LeadingTrivia
	node := p.sink.Store(Node{Kind: KindVariableDeclarator, Start: start, Leading: leading})

	tag := p.parseOptionalTagPrefix()
	if tag != p.sink.Nil() {
		p.sink.SetField(node, fieldTag, tag)
		p.sink.AddChild(node, tag)
	}

	if !isFunctionNameToken(p.curKind()) {
		p.emitMissing(DiagnosticMissingIdentifier, "expected declarator name", token.Identifier)
		p.sink.SetHasError(node, true)
		return node
	}
	name := p.parseQualifiedIdentifier()
	p.sink.SetField(node, fieldName, name)
	p.sink.AddChild(node, name)
	p.sink.SetEnd(node, p.sink.End(name))
	p.sink.SetTrailing(node, p.sink.Trailing(name))
	if p.at(token.Lt) {
		selector := p.parseStateSelector()
		p.sink.SetField(node, fieldCapacity, selector)
		p.sink.AddChild(node, selector)
	}

	dims := p.parseDimensions()
	for _, d := range dims {
		p.sink.AddChild(node, d)
		p.sink.SetEnd(node, p.sink.End(d))
		p.sink.SetTrailing(node, p.sink.Trailing(d))
	}
	if len(dims) > 0 {
		p.sink.SetField(node, fieldArray, dims[0])
	}
	if p.at(token.Lt) {
		selector := p.parseStateSelector()
		p.sink.SetField(node, fieldCapacity, selector)
		p.sink.AddChild(node, selector)
	}

	if p.at(token.Assign) {
		p.advance()
		init := p.parseDeclaratorInitializer()
		p.sink.SetField(node, fieldInitializer, init)
		p.sink.AddChild(node, init)
		p.sink.SetEnd(node, p.sink.End(init))
		p.sink.SetTrailing(node, p.sink.Trailing(init))
	}
	return node
}

func (p *parser[N, S]) parseDeclaratorInitializer() N {
	if p.at(token.LBrace) {
		return p.parseArrayLiteral()
	}
	return p.parseAssignment()
}

func (p *parser[N, S]) parseEnumDeclaration(quals []N) N {
	start := p.cur().Start.Offset
	leading := p.cur().LeadingTrivia
	if len(quals) > 0 {
		start = p.sink.Start(quals[0])
		leading = p.sink.Leading(quals[0])
	}
	kw := p.advance()
	node := p.sink.Store(Node{Kind: KindEnumDeclaration, Tok: kw, Start: start, Leading: leading})
	for _, q := range quals {
		p.sink.AddChild(node, q)
	}

	if p.at(token.Identifier) {
		name := p.parseQualifiedIdentifier()
		p.sink.SetField(node, fieldName, name)
		p.sink.AddChild(node, name)
	}

	var tag N
	switch {
	case p.at(token.Colon) && p.peekKind(1) == token.LBrace:
		colon := p.advance()
		tag = p.directiveSpan(KindTaggedType, colon.Start.Offset, colon.End.Offset, colon.LeadingTrivia, colon.TrailingTrivia)
	case p.at(token.Colon) && p.peekKind(1) == token.Identifier:
		colon := p.advance()
		tag = p.sink.NewLeaf(KindTaggedType, p.advance())
		p.sink.SetStart(tag, colon.Start.Offset)
	default:
		tag = p.parseOptionalTagPrefix()
	}
	if tag != p.sink.Nil() {
		p.sink.SetField(node, fieldTag, tag)
		p.sink.AddChild(node, tag)
	}

	if increment := p.parseEnumIncrementClause(); increment != p.sink.Nil() {
		p.sink.SetField(node, fieldIncrement, increment)
		p.sink.AddChild(node, increment)
	}

	if !p.at(token.LBrace) {
		p.emitMissing(DiagnosticMissingDeclaration, "expected enum body", token.LBrace)
		p.sink.SetHasError(node, true)
		return node
	}
	lb := p.advance()
	body := p.sink.Store(Node{Kind: KindBlock, Start: lb.Start.Offset, Leading: lb.LeadingTrivia})
	items := p.parseItemSequence(itemGrammar[N, S]{
		parseItem:      (*parser[N, S]).parseEnumEntry,
		stopKind:       token.RBrace,
		abortAtStop:    true,
		commaSeparated: true,
	})
	for _, it := range items {
		p.sink.AddChild(body, it)
	}
	if p.at(token.RBrace) {
		rb := p.advance()
		p.sink.SetEnd(body, rb.End.Offset)
		p.sink.SetTrailing(body, rb.TrailingTrivia)
	} else {
		p.sink.SetHasError(body, true)
		p.emitMissingToken(token.RBrace, "enum body")
	}
	p.sink.SetField(node, fieldBody, body)
	p.sink.AddChild(node, body)
	p.sink.SetEnd(node, p.sink.End(body))
	p.sink.SetTrailing(node, p.sink.Trailing(body))

	if p.at(token.Semicolon) {
		semi := p.advance()
		p.sink.SetEnd(node, semi.End.Offset)
		p.sink.SetTrailing(node, semi.TrailingTrivia)
	}
	return node
}

func (p *parser[N, S]) parseEnumIncrementClause() N {
	if !p.at(token.LParen) {
		return p.sink.Nil()
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
	p.sink.SetLeading(n, lp.LeadingTrivia)
	p.sink.SetTrailing(n, last.TrailingTrivia)
	return n
}

func (p *parser[N, S]) parseEnumEntry() N {
	start := p.cur().Start.Offset
	leading := p.cur().LeadingTrivia
	node := p.sink.Store(Node{Kind: KindEnumEntry, Start: start, Leading: leading})

	tag := p.parseOptionalTagPrefix()
	if tag != p.sink.Nil() {
		p.sink.SetField(node, fieldTag, tag)
		p.sink.AddChild(node, tag)
	}

	if !p.at(token.Identifier) {
		p.emitMissing(DiagnosticMissingIdentifier, "expected enum entry name", token.Identifier)
		p.sink.SetHasError(node, true)
		if !p.atEnd() && p.curKind() != token.Comma && p.curKind() != token.RBrace {
			bad := p.advance()
			p.sink.SetEnd(node, bad.End.Offset)
			p.sink.SetTrailing(node, bad.TrailingTrivia)
		}
		return node
	}
	name := p.parseQualifiedIdentifier()
	p.sink.SetField(node, fieldName, name)
	p.sink.AddChild(node, name)
	p.sink.SetEnd(node, p.sink.End(name))
	p.sink.SetTrailing(node, p.sink.Trailing(name))

	dims := p.parseDimensions()
	for _, d := range dims {
		p.sink.AddChild(node, d)
		p.sink.SetEnd(node, p.sink.End(d))
		p.sink.SetTrailing(node, p.sink.Trailing(d))
	}
	if len(dims) > 0 {
		p.sink.SetField(node, fieldArray, dims[0])
	}

	if p.at(token.Assign) {
		p.advance()
		val := p.parseTernary()
		p.sink.SetField(node, fieldValue, val)
		p.sink.AddChild(node, val)
		p.sink.SetEnd(node, p.sink.End(val))
		p.sink.SetTrailing(node, p.sink.Trailing(val))
	}
	return node
}
