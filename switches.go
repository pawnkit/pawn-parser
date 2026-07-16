package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser[N, S]) parseSwitchStatement() N {
	kw := p.advance()
	condition := p.parseParenCondition()
	node := p.sink.Store(Node{Kind: KindSwitchStatement, Tok: kw, Start: kw.Start.Offset, Leading: kw.LeadingTrivia})
	p.sink.SetField(node, fieldCondition, condition)
	p.sink.AddChild(node, condition)

	if !p.at(token.LBrace) {
		p.sink.SetHasError(node, true)
		return node
	}
	p.advance() // '{'
	clauses := p.parseItemSequence(itemGrammar[N, S]{
		parseMode:   itemParseSwitchClause,
		stopKind:    token.RBrace,
		abortAtStop: true,
	})
	for _, c := range clauses {
		p.sink.AddChild(node, c)
	}
	if p.at(token.RBrace) {
		rb := p.advance()
		p.sink.SetEnd(node, rb.End.Offset)
		p.sink.SetTrailing(node, rb.TrailingTrivia)
	} else {
		p.sink.SetHasError(node, true)
	}
	return node
}

func (p *parser[N, S]) parseSwitchClause() N {
	if p.at(token.KwCase) {
		kw := p.advance()
		wasSuppressed := p.suppressTagCast
		p.suppressTagCast = true
		values := p.parseCaseValueList()
		p.suppressTagCast = wasSuppressed
		node := p.sink.Store(Node{Kind: KindCaseClause, Tok: kw, Start: kw.Start.Offset, Leading: kw.LeadingTrivia})
		p.sink.SetField(node, fieldValues, values)
		p.sink.AddChild(node, values)
		if p.at(token.Colon) {
			p.advance()
		} else {
			p.sink.SetHasError(node, true)
		}
		body := p.parseClauseBody()
		p.sink.SetField(node, fieldBody, body)
		p.sink.AddChild(node, body)
		return node
	}
	if p.at(token.KwDefault) {
		kw := p.advance()
		node := p.sink.Store(Node{Kind: KindDefaultClause, Tok: kw, Start: kw.Start.Offset, Leading: kw.LeadingTrivia})
		if p.at(token.Colon) {
			p.advance()
		} else {
			p.sink.SetHasError(node, true)
		}
		body := p.parseClauseBody()
		p.sink.SetField(node, fieldBody, body)
		p.sink.AddChild(node, body)
		return node
	}
	tok := p.advance()
	n := p.sink.NewLeaf(KindRaw, tok)
	p.sink.SetHasError(n, true)
	return n
}

func (p *parser[N, S]) parseClauseBody() N {
	if p.at(token.LBrace) {
		return p.parseBlock()
	}
	if p.at(token.KwCase) || p.at(token.KwDefault) || p.at(token.RBrace) {
		tok := p.cur()
		return p.sink.Store(Node{Kind: KindEmptyStatement, Start: tok.Start.Offset, End: tok.Start.Offset, Leading: tok.LeadingTrivia})
	}
	return p.parseStatement()
}

func (p *parser[N, S]) parseCaseValueList() N {
	list := p.sink.Store(Node{Kind: KindCaseValueList, Start: p.cur().Start.Offset, Leading: p.cur().LeadingTrivia})
	for {
		v := p.parseCaseValue()
		if p.at(token.Comma) {
			p.mergeCommaTrivia(v, p.advance())
			p.sink.AddChild(list, v)
			continue
		}
		p.sink.AddChild(list, v)
		break
	}
	return list
}

func (p *parser[N, S]) parseCaseValue() N {
	start := p.parseTernary()
	if p.at(token.DotDot) {
		p.advance()
		end := p.parseTernary()
		node := p.sink.NewNode(KindCaseRange, start, end)
		p.sink.SetField(node, fieldStart, start)
		p.sink.SetField(node, fieldEnd, end)
		return node
	}
	return start
}
