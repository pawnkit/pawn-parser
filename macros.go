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
		node := p.storeNode(Node{Kind: KindDirectiveDefine, Start: startOffset, End: nameNode.End, Leading: leading, Trailing: p.toks[p.pos-1].TrailingTrivia})
		p.setField(node, fieldName, nameNode)
		p.addChild(node, nameNode)
		if params != nil {
			p.setField(node, fieldParameters, params)
			p.addChild(node, params)
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

	node := p.storeNode(Node{Kind: KindDirectiveDefine, Start: startOffset, End: bodyEnd, Leading: leading, Trailing: lastTok.TrailingTrivia})
	p.setField(node, fieldName, nameNode)
	p.addChild(node, nameNode)
	if params != nil {
		p.setField(node, fieldParameters, params)
		p.addChild(node, params)
	}
	p.setField(node, fieldValue, valueNode)
	p.addChild(node, valueNode)
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
	n := p.directiveSpan(kind, startOffset, end, leading, last.TrailingTrivia)
	n.HasError = true
	return n
}

func (p *parser) parseMacroParamList() (*Node, bool) {
	startIdx := p.pos
	lp := p.advance() // '('
	params := p.storeNode(Node{Kind: KindParameterList, Start: lp.Start.Offset, Leading: lp.LeadingTrivia})

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
		p.addChild(params, p.newLeaf(KindIdentifier, tok))

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
		return p.directiveSpan(KindMacroBody, bodyStart, bodyEnd, nil, nil)
	}
	if bodyStartIdx >= bodyEndIdx {
		return raw()
	}

	bodyToks := make([]token.Token, bodyEndIdx-bodyStartIdx, bodyEndIdx-bodyStartIdx+1)
	copy(bodyToks, p.toks[bodyStartIdx:bodyEndIdx])
	last := bodyToks[len(bodyToks)-1]
	bodyToks = append(bodyToks, token.Token{Kind: token.EOF, Start: last.End, End: last.End})

	if expr, ok := p.tryParseAll(bodyToks, false, (*parser).parseExpression); ok {
		return expr
	}
	if stmt, ok := p.tryParseAll(bodyToks, true, (*parser).parseStatement); ok {
		if childrenHaveMissingSemicolon(stmt) {
			return raw()
		}
		return stmt
	}
	return raw()
}

func childrenHaveMissingSemicolon(node *Node) bool {
	for _, child := range node.Children {
		if child.MissingSemi || childrenHaveMissingSemicolon(child) {
			return true
		}
	}
	return false
}

func (p *parser) tryParseAll(toks []token.Token, lenientTrailingSemi bool, fn func(*parser) *Node) (*Node, bool) {
	mark := p.storage.mark()
	sub := parser{
		source: p.source, toks: toks, storage: p.storage,
		allowMissingTrailingSemi: lenientTrailingSemi,
	}
	node := fn(&sub)
	if node == nil || node.HasError || sub.broken || !sub.atEnd() {
		p.storage.rewind(mark)
		return nil, false
	}
	return node, true
}
