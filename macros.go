package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser[N, S]) parseDefineDirective(startOffset int) N {
	leading := p.cur().LeadingTrivia
	p.advance() // '#'
	p.advance() // 'define'

	if !p.at(token.Identifier) {
		return p.consumeRawDirectiveLineFrom(startOffset, KindDirectiveDefine, leading)
	}
	nameTok := p.advance()
	nameNode := p.sink.NewLeaf(KindIdentifier, nameTok)

	var params N
	if p.at(token.LParen) && len(nameTok.TrailingTrivia) == 0 {
		if pl, ok := p.parseMacroParamList(); ok {
			params = pl
		}
	}

	if p.atEnd() || lastTokenEndsLine(p.toks[p.pos-1]) {
		node := p.sink.Store(Node{Kind: KindDirectiveDefine, Start: startOffset, End: p.sink.End(nameNode), Leading: leading, Trailing: p.toks[p.pos-1].TrailingTrivia})
		p.sink.SetField(node, fieldName, nameNode)
		p.sink.AddChild(node, nameNode)
		if params != p.sink.Nil() {
			p.sink.SetField(node, fieldParameters, params)
			p.sink.AddChild(node, params)
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

	node := p.sink.Store(Node{Kind: KindDirectiveDefine, Start: startOffset, End: bodyEnd, Leading: leading, Trailing: lastTok.TrailingTrivia})
	p.sink.SetField(node, fieldName, nameNode)
	p.sink.AddChild(node, nameNode)
	if params != p.sink.Nil() {
		p.sink.SetField(node, fieldParameters, params)
		p.sink.AddChild(node, params)
	}
	p.sink.SetField(node, fieldValue, valueNode)
	p.sink.AddChild(node, valueNode)
	return node
}

func (p *parser[N, S]) consumeRawDirectiveLineFrom(startOffset int, kind Kind, leading []token.Trivia) N {
	var last token.Token
	for !p.atEnd() {
		last = p.advance()
		if lastTokenEndsLine(last) {
			break
		}
	}
	end := max(last.End.Offset, startOffset)
	n := p.directiveSpan(kind, startOffset, end, leading, last.TrailingTrivia)
	p.sink.SetHasError(n, true)
	return n
}

func (p *parser[N, S]) parseMacroParamList() (N, bool) {
	startIdx := p.pos
	lp := p.advance() // '('
	params := p.sink.Store(Node{Kind: KindParameterList, Start: lp.Start.Offset, Leading: lp.LeadingTrivia})

	if p.at(token.RParen) {
		rp := p.advance()
		p.sink.SetEnd(params, rp.End.Offset)
		p.sink.SetTrailing(params, rp.TrailingTrivia)
		return params, true
	}

	for {
		if !p.at(token.MacroParam) && !p.at(token.Identifier) {
			p.pos = startIdx
			return p.sink.Nil(), false
		}
		tok := p.advance()
		p.sink.AddChild(params, p.sink.NewLeaf(KindIdentifier, tok))

		if p.at(token.Comma) {
			p.advance()
			continue
		}
		if p.at(token.RParen) {
			rp := p.advance()
			p.sink.SetEnd(params, rp.End.Offset)
			p.sink.SetTrailing(params, rp.TrailingTrivia)
			return params, true
		}
		p.pos = startIdx
		return p.sink.Nil(), false
	}
}

func (p *parser[N, S]) parseMacroBody(bodyStartIdx, bodyEndIdx, bodyStart, bodyEnd int) N {
	raw := func() N {
		return p.directiveSpan(KindMacroBody, bodyStart, bodyEnd, nil, nil)
	}
	if bodyStartIdx >= bodyEndIdx {
		return raw()
	}

	bodyToks := make([]token.Token, bodyEndIdx-bodyStartIdx, bodyEndIdx-bodyStartIdx+1)
	copy(bodyToks, p.toks[bodyStartIdx:bodyEndIdx])
	last := bodyToks[len(bodyToks)-1]
	bodyToks = append(bodyToks, token.Token{Kind: token.EOF, Start: last.End, End: last.End})

	if expr, ok := p.tryParseAll(bodyToks, false, (*parser[N, S]).parseExpression); ok {
		return expr
	}
	if stmt, ok := p.tryParseAll(bodyToks, true, (*parser[N, S]).parseStatement); ok {
		if p.childrenHaveMissingSemicolon(stmt) {
			return raw()
		}
		return stmt
	}
	return raw()
}

func (p *parser[N, S]) childrenHaveMissingSemicolon(node N) bool {
	for _, child := range p.sink.Children(node) {
		if p.sink.MissingSemi(child) || p.childrenHaveMissingSemicolon(child) {
			return true
		}
	}
	return false
}

func (p *parser[N, S]) tryParseAll(toks []token.Token, lenientTrailingSemi bool, fn func(*parser[N, S]) N) (N, bool) {
	mark := p.sink.Mark()
	sub := parser[N, S]{
		source: p.source, toks: toks,
		sink: p.sink, allowMissingTrailingSemi: lenientTrailingSemi,
	}
	node := fn(&sub)
	if node == p.sink.Nil() || p.sink.HasError(node) || sub.broken || !sub.atEnd() {
		p.sink.Rewind(mark)
		return p.sink.Nil(), false
	}
	return node, true
}
