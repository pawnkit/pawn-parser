package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser[N, S]) parseParameterList() N {
	if !p.at(token.LParen) {
		p.emitMissingToken(token.LParen, "parameter list")
		n := p.sink.Store(Node{Kind: KindParameterList, HasError: true})
		return n
	}
	lp := p.advance()
	node := p.sink.Store(Node{Kind: KindParameterList, Start: lp.Start.Offset, Leading: lp.LeadingTrivia})
	items := p.parseItemSequence(itemGrammar[N, S]{
		parseItem:      (*parser[N, S]).parseParameter,
		stopKind:       token.RParen,
		abortAtStop:    true,
		commaSeparated: true,
	})
	for _, it := range items {
		p.sink.AddChild(node, it)
	}
	if p.at(token.RParen) {
		rp := p.advance()
		p.sink.SetEnd(node, rp.End.Offset)
		p.sink.SetTrailing(node, rp.TrailingTrivia)
	} else {
		p.sink.SetHasError(node, true)
		p.emitMissingToken(token.RParen, "parameter list")
	}
	return node
}

func (p *parser[N, S]) parseParameter() N {
	if p.at(token.Ellipsis) {
		tok := p.advance()
		return p.sink.NewLeaf(KindParameter, tok)
	}

	start := p.cur().Start.Offset
	leading := p.cur().LeadingTrivia
	node := p.sink.Store(Node{Kind: KindParameter, Start: start, Leading: leading})
	p.parseParameterQualifiers(node)

	if p.at(token.Amp) {
		p.advance()
	}

	tag := p.parseOptionalTagPrefix()
	if tag != p.sink.Nil() {
		p.sink.SetField(node, fieldTag, tag)
		p.sink.AddChild(node, tag)
	}

	if p.at(token.Amp) {
		p.advance()
	}

	if p.at(token.Ellipsis) {
		tok := p.advance()
		p.sink.SetEnd(node, tok.End.Offset)
		p.sink.SetTrailing(node, tok.TrailingTrivia)
		return node
	}

	if !p.parseParameterName(node) {
		return node
	}
	p.parseParameterSuffix(node)
	return node
}

func (p *parser[N, S]) parseParameterQualifiers(node N) {
	for p.at(token.KwConst) || p.at(token.KwStock) {
		p.sink.AddChild(node, p.sink.NewLeaf(KindIdentifier, p.advance()))
	}
}

func (p *parser[N, S]) parseParameterName(node N) bool {
	if !isFunctionNameToken(p.curKind()) {
		p.emitMissing(DiagnosticMissingIdentifier, "expected parameter name", token.Identifier)
		p.sink.SetHasError(node, true)
		if !p.atEnd() && p.curKind() != token.Comma && p.curKind() != token.RParen {
			bad := p.advance()
			p.sink.SetEnd(node, bad.End.Offset)
			p.sink.SetTrailing(node, bad.TrailingTrivia)
		}
		return false
	}
	name := p.parseQualifiedIdentifier()
	p.sink.SetField(node, fieldName, name)
	p.sink.AddChild(node, name)
	p.sink.SetEnd(node, p.sink.End(name))
	p.sink.SetTrailing(node, p.sink.Trailing(name))
	return true
}

func (p *parser[N, S]) parseParameterSuffix(node N) {
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
		generic := p.parseStateSelector()
		p.sink.SetField(node, fieldGeneric, generic)
		p.sink.AddChild(node, generic)
	}

	if p.at(token.Assign) {
		p.advance()
		def := p.parseAssignment()
		p.sink.SetField(node, fieldDefaultValue, def)
		p.sink.AddChild(node, def)
		p.sink.SetEnd(node, p.sink.End(def))
		p.sink.SetTrailing(node, p.sink.Trailing(def))
	}
}
