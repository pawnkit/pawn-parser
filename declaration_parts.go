package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser[N, S]) parseStateSelector() N {
	if !p.at(token.Lt) {
		return p.sink.Nil()
	}
	startIdx := p.pos
	mark := p.sink.Mark()
	lt := p.advance()
	node := p.sink.Store(Node{Kind: KindTaggedType, Start: lt.Start.Offset, Leading: lt.LeadingTrivia})
	for !p.at(token.Gt) {
		if p.curKind() != token.Identifier && !isKeywordToken(p.curKind()) {
			p.pos = startIdx
			p.sink.Rewind(mark)
			return p.rawStateSelector()
		}
		p.sink.AddChild(node, p.sink.NewLeaf(KindIdentifier, p.advance()))
		if p.at(token.Comma) {
			p.advance()
			continue
		}
		break
	}
	if !p.at(token.Gt) {
		p.pos = startIdx
		p.sink.Rewind(mark)
		return p.rawStateSelector()
	}
	gt := p.advance()
	p.sink.SetEnd(node, gt.End.Offset)
	p.sink.SetTrailing(node, gt.TrailingTrivia)
	return node
}

func (p *parser[N, S]) rawStateSelector() N {
	stateStart := p.pos
	p.skipAngleStateSelector()
	n := p.directiveSpan(KindStateSelector, p.toks.at(stateStart).Start.Offset, p.toks.endOffset(p.pos-1), nil, nil)
	return n
}

func (p *parser[N, S]) skipAngleStateSelector() {
	depth := 0
	for !p.atEnd() {
		switch p.curKind() {
		case token.Lt:
			depth++
			p.advance()
		case token.Gt:
			depth--
			p.advance()
			if depth <= 0 {
				return
			}
		case token.Semicolon, token.LBrace:
			return
		default:
			p.advance()
		}
	}
}

func (p *parser[N, S]) parseOptionalTagPrefix() N {
	if p.qualifiedTagPrefixStart() {
		name := p.parseQualifiedIdentifier()
		start, end := clampRange(p.source, p.sink.Start(name), p.sink.End(name))
		p.rememberTag(string(p.source[start:end]))
		colon := p.advance()
		node := p.sink.Store(Node{Kind: KindTaggedType, Start: p.sink.Start(name), End: colon.End.Offset, Leading: p.sink.Leading(name), Trailing: colon.TrailingTrivia})
		p.sink.AddChild(node, name)
		p.sink.SetEnd(node, colon.End.Offset)
		p.sink.SetTrailing(node, colon.TrailingTrivia)
		return node
	}
	if p.curKind() == token.Identifier && p.peekKind(1) == token.Colon {
		tagTok := p.advance()
		p.rememberTag(tagTok.Text(p.source))
		colon := p.advance()
		node := p.sink.Store(Node{Kind: KindTaggedType, Start: tagTok.Start.Offset, Leading: tagTok.LeadingTrivia})
		p.sink.AddChild(node, p.sink.NewLeaf(KindIdentifier, tagTok))
		p.sink.SetEnd(node, colon.End.Offset)
		p.sink.SetTrailing(node, colon.TrailingTrivia)
		return node
	}
	if p.curKind() == token.LBrace {
		saved := p.pos
		mark := p.sink.Mark()
		lb := p.advance()
		node := p.sink.Store(Node{Kind: KindTaggedType, Start: lb.Start.Offset, Leading: lb.LeadingTrivia})
		for {
			if !p.at(token.Identifier) {
				p.pos = saved
				p.sink.Rewind(mark)
				return p.sink.Nil()
			}
			p.sink.AddChild(node, p.sink.NewLeaf(KindIdentifier, p.advance()))
			if p.at(token.Comma) {
				p.advance()
				continue
			}
			break
		}
		if !p.at(token.RBrace) {
			p.pos = saved
			p.sink.Rewind(mark)
			return p.sink.Nil()
		}
		p.advance()
		if !p.at(token.Colon) {
			p.pos = saved
			p.sink.Rewind(mark)
			return p.sink.Nil()
		}
		colon := p.advance()
		p.sink.SetEnd(node, colon.End.Offset)
		p.sink.SetTrailing(node, colon.TrailingTrivia)
		return node
	}
	return p.sink.Nil()
}

func (p *parser[N, S]) qualifiedTagPrefixStart() bool {
	if !p.at(token.Identifier) || p.peekKind(1) != token.ColonColon {
		return false
	}
	i := 1
	for p.peekKind(i) == token.ColonColon && p.peekKind(i+1) == token.Identifier {
		i += 2
	}
	return p.peekKind(i) == token.Colon
}

func (p *parser[N, S]) parseQualifiedIdentifier() N {
	name := p.sink.NewLeaf(KindIdentifier, p.advance())
	for p.at(token.ColonColon) {
		name = p.parseMemberSelection(name)
	}
	return name
}

func (p *parser[N, S]) rememberTag(name string) {
	if p.knownTags == nil {
		p.knownTags = make(map[string]struct{})
	}
	p.knownTags[name] = struct{}{}
}

func (p *parser[N, S]) knowsTag(name string) bool {
	_, ok := p.knownTags[name]
	return ok
}

func (p *parser[N, S]) parseDimensions() []N {
	var dims []N
	for p.at(token.LBracket) {
		lb := p.advance()
		dim := p.sink.Store(Node{Kind: KindDimension, Start: lb.Start.Offset, Leading: lb.LeadingTrivia})
		if !p.at(token.RBracket) {
			expr := p.parseExpression()
			p.sink.SetField(dim, fieldSize, expr)
			p.sink.AddChild(dim, expr)
		}
		if p.at(token.Identifier) && p.cur().Text(p.source) == "char" {
			packed := p.sink.NewLeaf(KindIdentifier, p.advance())
			p.sink.SetField(dim, fieldPacked, packed)
			p.sink.AddChild(dim, packed)
		}
		if p.at(token.RBracket) {
			rb := p.advance()
			p.sink.SetEnd(dim, rb.End.Offset)
			p.sink.SetTrailing(dim, rb.TrailingTrivia)
		} else {
			p.sink.SetHasError(dim, true)
			p.emitMissingToken(token.RBracket, "array dimension")
		}
		dims = p.sink.Append(dims, dim)
	}
	return dims
}
