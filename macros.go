package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser) parseDefineDirective(startOffset int) *Node {
	leading := p.cur().LeadingTrivia
	p.advance() // '#'
	p.advance() // 'define'

	if !p.at(token.Identifier) {
		return p.consumeRawDirectiveLineFrom(startOffset, KindDirectiveDefine, leading)
	}
	nameTok := p.advance()
	nameNode := p.newLeaf(KindIdentifier, nameTok)

	var params *Node
	if p.at(token.LParen) && len(nameTok.TrailingTrivia) == 0 {
		if pl, ok := p.parseMacroParamList(); ok {
			params = pl
		}
	}

	if p.atEnd() || lastTokenEndsLine(p.toks[p.pos-1]) {
		node := &Node{Kind: KindDirectiveDefine, Start: startOffset, End: nameNode.End, Leading: leading, Trailing: p.toks[p.pos-1].TrailingTrivia}
		setField(node, "name", nameNode)
		node.addChild(nameNode)
		if params != nil {
			setField(node, "parameters", params)
			node.addChild(params)
		}
		return node
	}

	bodyStartIdx := p.pos
	bodyStart := p.cur().Start.Offset
	var lastTok token.Token
	for !p.atEnd() {
		lastTok = p.advance()
		if lastTokenEndsLine(lastTok) {
			break
		}
	}
	bodyEndIdx := p.pos
	bodyEnd := lastTok.End.Offset

	valueNode := p.parseMacroBody(bodyStartIdx, bodyEndIdx, bodyStart, bodyEnd)

	node := &Node{Kind: KindDirectiveDefine, Start: startOffset, End: bodyEnd, Leading: leading, Trailing: lastTok.TrailingTrivia}
	setField(node, "name", nameNode)
	node.addChild(nameNode)
	if params != nil {
		setField(node, "parameters", params)
		node.addChild(params)
	}
	setField(node, "value", valueNode)
	node.addChild(valueNode)
	return node
}

func (p *parser) consumeRawDirectiveLineFrom(startOffset int, kind Kind, leading []token.Trivia) *Node {
	var last token.Token
	for !p.atEnd() {
		last = p.advance()
		if lastTokenEndsLine(last) {
			break
		}
	}
	end := max(last.End.Offset, startOffset)
	n := directiveSpan(p.source, kind, startOffset, end, leading, last.TrailingTrivia)
	n.HasError = true
	return n
}

func (p *parser) parseMacroParamList() (*Node, bool) {
	startIdx := p.pos
	lp := p.advance() // '('
	params := &Node{Kind: KindParameterList, Start: lp.Start.Offset, Leading: lp.LeadingTrivia}

	if p.at(token.RParen) {
		rp := p.advance()
		params.End = rp.End.Offset
		params.Trailing = rp.TrailingTrivia
		return params, true
	}

	for {
		if !p.at(token.MacroParam) && !p.at(token.Identifier) {
			p.pos = startIdx
			return nil, false
		}
		tok := p.advance()
		params.addChild(p.newLeaf(KindIdentifier, tok))

		if p.at(token.Comma) {
			p.advance()
			continue
		}
		if p.at(token.RParen) {
			rp := p.advance()
			params.End = rp.End.Offset
			params.Trailing = rp.TrailingTrivia
			return params, true
		}
		p.pos = startIdx
		return nil, false
	}
}

func (p *parser) parseMacroBody(bodyStartIdx, bodyEndIdx, bodyStart, bodyEnd int) *Node {
	raw := func() *Node {
		n := rawNode(p.source, bodyStart, bodyEnd)
		n.HasError = false
		return n
	}
	if bodyStartIdx >= bodyEndIdx {
		return raw()
	}

	bodyToks := make([]token.Token, bodyEndIdx-bodyStartIdx, bodyEndIdx-bodyStartIdx+1)
	copy(bodyToks, p.toks[bodyStartIdx:bodyEndIdx])
	last := bodyToks[len(bodyToks)-1]
	bodyToks = append(bodyToks, token.Token{Kind: token.EOF, Start: last.End, End: last.End})

	if expr, ok := tryParseAll(bodyToks, p.source, false, (*parser).parseExpression); ok {
		return expr
	}
	if stmt, ok := tryParseAll(bodyToks, p.source, true, (*parser).parseStatement); ok {
		return stmt
	}
	return raw()
}

func tryParseAll(toks []token.Token, source []byte, lenientTrailingSemi bool, fn func(*parser) *Node) (*Node, bool) {
	sub := &parser{source: source, toks: toks, allowMissingTrailingSemi: lenientTrailingSemi}
	node := fn(sub)
	if node == nil || node.HasError || sub.broken || !sub.atEnd() {
		return nil, false
	}
	return node, true
}
