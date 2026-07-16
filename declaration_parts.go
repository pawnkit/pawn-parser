package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser) parseStateSelector() *Node {
	if !p.at(token.Lt) {
		return nil
	}
	startIdx := p.pos
	lt := p.advance()
	node := p.storeNode(Node{Kind: KindTaggedType, Start: lt.Start.Offset, Leading: lt.LeadingTrivia})
	for !p.at(token.Gt) {
		if p.curKind() != token.Identifier && !isKeywordToken(p.curKind()) {
			p.pos = startIdx
			return p.rawStateSelector()
		}
		p.addChild(node, p.newLeaf(KindIdentifier, p.advance()))
		if p.at(token.Comma) {
			p.advance()
			continue
		}
		break
	}
	if !p.at(token.Gt) {
		p.pos = startIdx
		return p.rawStateSelector()
	}
	gt := p.advance()
	node.End = gt.End.Offset
	node.Trailing = gt.TrailingTrivia
	return node
}

func (p *parser) rawStateSelector() *Node {
	stateStart := p.pos
	p.skipAngleStateSelector()
	n := p.directiveSpan(KindStateSelector, p.toks[stateStart].Start.Offset, p.toks[p.pos-1].End.Offset, nil, nil)
	return n
}

func (p *parser) skipAngleStateSelector() {
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

func (p *parser) parseOptionalTagPrefix() *Node {
	if p.qualifiedTagPrefixStart() {
		name := p.parseQualifiedIdentifier()
		p.rememberTag(name.Text(p.source))
		colon := p.advance()
		node := p.storeNode(Node{Kind: KindTaggedType, Start: name.Start, End: colon.End.Offset, Leading: name.Leading, Trailing: colon.TrailingTrivia})
		p.addChild(node, name)
		node.End = colon.End.Offset
		node.Trailing = colon.TrailingTrivia
		return node
	}
	if p.curKind() == token.Identifier && p.peekKind(1) == token.Colon {
		tagTok := p.advance()
		p.rememberTag(tagTok.Text(p.source))
		colon := p.advance()
		node := p.storeNode(Node{Kind: KindTaggedType, Start: tagTok.Start.Offset, Leading: tagTok.LeadingTrivia})
		p.addChild(node, p.newLeaf(KindIdentifier, tagTok))
		node.End = colon.End.Offset
		node.Trailing = colon.TrailingTrivia
		return node
	}
	if p.curKind() == token.LBrace {
		saved := p.pos
		lb := p.advance()
		node := p.storeNode(Node{Kind: KindTaggedType, Start: lb.Start.Offset, Leading: lb.LeadingTrivia})
		for {
			if !p.at(token.Identifier) {
				p.pos = saved
				return nil
			}
			p.addChild(node, p.newLeaf(KindIdentifier, p.advance()))
			if p.at(token.Comma) {
				p.advance()
				continue
			}
			break
		}
		if !p.at(token.RBrace) {
			p.pos = saved
			return nil
		}
		p.advance()
		if !p.at(token.Colon) {
			p.pos = saved
			return nil
		}
		colon := p.advance()
		node.End = colon.End.Offset
		node.Trailing = colon.TrailingTrivia
		return node
	}
	return nil
}

func (p *parser) qualifiedTagPrefixStart() bool {
	if !p.at(token.Identifier) || p.peekKind(1) != token.ColonColon {
		return false
	}
	i := 1
	for p.peekKind(i) == token.ColonColon && p.peekKind(i+1) == token.Identifier {
		i += 2
	}
	return p.peekKind(i) == token.Colon
}

func (p *parser) parseQualifiedIdentifier() *Node {
	name := p.newLeaf(KindIdentifier, p.advance())
	for p.at(token.ColonColon) {
		name = p.parseMemberSelection(name)
	}
	return name
}

func (p *parser) rememberTag(name string) {
	if p.knownTags == nil {
		p.knownTags = make(map[string]struct{})
	}
	p.knownTags[name] = struct{}{}
}

func (p *parser) knowsTag(name string) bool {
	_, ok := p.knownTags[name]
	return ok
}

func (p *parser) parseDimensions() []*Node {
	var dims []*Node
	for p.at(token.LBracket) {
		lb := p.advance()
		dim := p.storeNode(Node{Kind: KindDimension, Start: lb.Start.Offset, Leading: lb.LeadingTrivia})
		if !p.at(token.RBracket) {
			expr := p.parseExpression()
			p.setField(dim, "size", expr)
			p.addChild(dim, expr)
		}
		if p.at(token.Identifier) && p.cur().Text(p.source) == "char" {
			packed := p.newLeaf(KindIdentifier, p.advance())
			p.setField(dim, "packed", packed)
			p.addChild(dim, packed)
		}
		if p.at(token.RBracket) {
			rb := p.advance()
			dim.End = rb.End.Offset
			dim.Trailing = rb.TrailingTrivia
		} else {
			dim.HasError = true
			p.emitMissingToken(token.RBracket, "array dimension")
		}
		dims = p.appendNode(dims, dim)
	}
	return dims
}
