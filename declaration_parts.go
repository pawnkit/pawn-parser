package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser) parseStateSelector() *Node {
	if !p.at(token.Lt) {
		return nil
	}
	startIdx := p.pos
	lt := p.advance()
	node := &Node{Kind: KindTaggedType, Start: lt.Start.Offset, Leading: lt.LeadingTrivia}
	for !p.at(token.Gt) {
		if p.cur().Kind != token.Identifier && !isKeywordToken(p.cur().Kind) {
			p.pos = startIdx
			return p.rawStateSelector()
		}
		node.addChild(p.newLeaf(KindIdentifier, p.advance()))
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
	n := directiveSpan(p.source, KindStateSelector, p.toks[stateStart].Start.Offset, p.toks[p.pos-1].End.Offset, nil, nil)
	return n
}

func (p *parser) skipAngleStateSelector() {
	depth := 0
	for !p.atEnd() {
		switch p.cur().Kind {
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
		node := &Node{Kind: KindTaggedType, Start: name.Start, End: colon.End.Offset, Leading: name.Leading, Trailing: colon.TrailingTrivia}
		node.addChild(name)
		node.End = colon.End.Offset
		node.Trailing = colon.TrailingTrivia
		return node
	}
	if p.cur().Kind == token.Identifier && p.peek(1).Kind == token.Colon {
		tagTok := p.advance()
		p.rememberTag(tagTok.Text(p.source))
		colon := p.advance()
		node := &Node{Kind: KindTaggedType, Start: tagTok.Start.Offset, Leading: tagTok.LeadingTrivia}
		node.addChild(p.newLeaf(KindIdentifier, tagTok))
		node.End = colon.End.Offset
		node.Trailing = colon.TrailingTrivia
		return node
	}
	if p.cur().Kind == token.LBrace {
		saved := p.pos
		lb := p.advance()
		node := &Node{Kind: KindTaggedType, Start: lb.Start.Offset, Leading: lb.LeadingTrivia}
		for {
			if !p.at(token.Identifier) {
				p.pos = saved
				return nil
			}
			node.addChild(p.newLeaf(KindIdentifier, p.advance()))
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
	if !p.at(token.Identifier) || p.peek(1).Kind != token.ColonColon {
		return false
	}
	i := 1
	for p.peek(i).Kind == token.ColonColon && p.peek(i+1).Kind == token.Identifier {
		i += 2
	}
	return p.peek(i).Kind == token.Colon
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
		dim := &Node{Kind: KindDimension, Start: lb.Start.Offset, Leading: lb.LeadingTrivia}
		if !p.at(token.RBracket) {
			expr := p.parseExpression()
			setField(dim, "size", expr)
			dim.addChild(expr)
		}
		if p.at(token.Identifier) && p.cur().Text(p.source) == "char" {
			packed := p.newLeaf(KindIdentifier, p.advance())
			setField(dim, "packed", packed)
			dim.addChild(packed)
		}
		if p.at(token.RBracket) {
			rb := p.advance()
			dim.End = rb.End.Offset
			dim.Trailing = rb.TrailingTrivia
		} else {
			dim.HasError = true
		}
		dims = append(dims, dim)
	}
	return dims
}
