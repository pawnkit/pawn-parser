package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser) parseSwitchStatement() *Node {
	kw := p.advance()
	condition := p.parseParenCondition()
	node := p.storeNode(Node{Kind: KindSwitchStatement, Tok: kw, Start: kw.Start.Offset, Leading: kw.LeadingTrivia})
	p.setField(node, "condition", condition)
	p.addChild(node, condition)

	if !p.at(token.LBrace) {
		node.HasError = true
		return node
	}
	p.advance() // '{'
	clauses := p.parseItemSequence(itemGrammar{
		parseItem: func(p *parser) *Node { return p.parseSwitchClause() },
		stop: func(p *parser) bool {
			p.abortIfSharedAcrossBranch()
			return p.at(token.RBrace)
		},
	})
	for _, c := range clauses {
		p.addChild(node, c)
	}
	if p.at(token.RBrace) {
		rb := p.advance()
		node.End = rb.End.Offset
		node.Trailing = rb.TrailingTrivia
	} else {
		node.HasError = true
	}
	return node
}

func (p *parser) parseSwitchClause() *Node {
	if p.at(token.KwCase) {
		kw := p.advance()
		wasSuppressed := p.suppressTagCast
		p.suppressTagCast = true
		values := p.parseCaseValueList()
		p.suppressTagCast = wasSuppressed
		node := p.storeNode(Node{Kind: KindCaseClause, Tok: kw, Start: kw.Start.Offset, Leading: kw.LeadingTrivia})
		p.setField(node, "values", values)
		p.addChild(node, values)
		if p.at(token.Colon) {
			p.advance()
		} else {
			node.HasError = true
		}
		body := p.parseClauseBody()
		p.setField(node, "body", body)
		p.addChild(node, body)
		return node
	}
	if p.at(token.KwDefault) {
		kw := p.advance()
		node := p.storeNode(Node{Kind: KindDefaultClause, Tok: kw, Start: kw.Start.Offset, Leading: kw.LeadingTrivia})
		if p.at(token.Colon) {
			p.advance()
		} else {
			node.HasError = true
		}
		body := p.parseClauseBody()
		p.setField(node, "body", body)
		p.addChild(node, body)
		return node
	}
	tok := p.advance()
	n := p.newLeaf(KindRaw, tok)
	n.HasError = true
	return n
}

func (p *parser) parseClauseBody() *Node {
	if p.at(token.LBrace) {
		return p.parseBlock()
	}
	if p.at(token.KwCase) || p.at(token.KwDefault) || p.at(token.RBrace) {
		tok := p.cur()
		return p.storeNode(Node{Kind: KindEmptyStatement, Start: tok.Start.Offset, End: tok.Start.Offset, Leading: tok.LeadingTrivia})
	}
	return p.parseStatement()
}

func (p *parser) parseCaseValueList() *Node {
	list := p.storeNode(Node{Kind: KindCaseValueList, Start: p.cur().Start.Offset, Leading: p.cur().LeadingTrivia})
	for {
		v := p.parseCaseValue()
		if p.at(token.Comma) {
			p.mergeCommaTrivia(v, p.advance())
			p.addChild(list, v)
			continue
		}
		p.addChild(list, v)
		break
	}
	return list
}

func (p *parser) parseCaseValue() *Node {
	start := p.parseTernary()
	if p.at(token.DotDot) {
		p.advance()
		end := p.parseTernary()
		node := p.newNode(KindCaseRange, start, end)
		p.setField(node, "start", start)
		p.setField(node, "end", end)
		return node
	}
	return start
}
