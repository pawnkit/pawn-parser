package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser[N, S]) parseReturnStatement() N {
	kw := p.advance()
	node := p.sink.Store(Node{Kind: KindReturnStatement, Tok: kw, Start: kw.Start.Offset, Leading: kw.LeadingTrivia})
	if !p.at(token.Semicolon) {
		val := p.parseExpression()
		p.sink.SetField(node, fieldValue, val)
		p.sink.AddChild(node, val)
	}
	p.consumeTrailingSemi(node)
	return node
}

func (p *parser[N, S]) simpleKeywordStatement(kind Kind) N {
	kw := p.advance()
	node := p.sink.Store(Node{Kind: kind, Tok: kw, Start: kw.Start.Offset, Leading: kw.LeadingTrivia})
	if p.at(token.Semicolon) {
		semi := p.advance()
		p.sink.SetEnd(node, semi.End.Offset)
		p.sink.SetTrailing(node, semi.TrailingTrivia)
	} else {
		p.sink.SetEnd(node, kw.End.Offset)
		p.sink.SetTrailing(node, kw.TrailingTrivia)
		if p.missingSemiOK() {
			p.sink.SetMissingSemi(node, true)
		} else {
			p.sink.SetHasError(node, true)
		}
	}
	return node
}

func (p *parser[N, S]) parseGotoStatement() N {
	kw := p.advance()
	node := p.sink.Store(Node{Kind: KindGotoStatement, Tok: kw, Start: kw.Start.Offset, Leading: kw.LeadingTrivia})
	if p.at(token.Identifier) {
		label := p.sink.NewLeaf(KindIdentifier, p.advance())
		p.sink.SetField(node, fieldLabel, label)
		p.sink.AddChild(node, label)
	} else {
		p.sink.SetHasError(node, true)
	}
	p.consumeTrailingSemi(node)
	return node
}

func (p *parser[N, S]) parseLabelStatement() N {
	nameTok := p.advance()
	name := p.sink.NewLeaf(KindIdentifier, nameTok)
	colon := p.advance() // ':'
	node := p.sink.Store(Node{Kind: KindLabelStatement, Start: nameTok.Start.Offset, Leading: nameTok.LeadingTrivia})
	p.sink.SetField(node, fieldLabel, name)
	p.sink.AddChild(node, name)
	p.sink.SetEnd(node, colon.End.Offset)
	p.sink.SetTrailing(node, colon.TrailingTrivia)
	return node
}

func (p *parser[N, S]) parseStateStatement() N {
	kw := p.advance()
	node := p.sink.Store(Node{Kind: KindStateStatement, Tok: kw, Start: kw.Start.Offset, Leading: kw.LeadingTrivia})
	if !p.at(token.Identifier) {
		p.sink.SetHasError(node, true)
		p.consumeTrailingSemi(node)
		return node
	}

	name := p.sink.NewLeaf(KindIdentifier, p.advance())
	p.sink.SetField(node, fieldState, name)
	p.sink.AddChild(node, name)
	p.parseStateStatementTarget(node)
	p.consumeTrailingSemi(node)
	return node
}

func (p *parser[N, S]) parseStateStatementTarget(node N) {
	if !p.at(token.Colon) {
		return
	}
	p.advance()
	if !p.at(token.Identifier) && !isKeywordToken(p.curKind()) {
		p.sink.SetHasError(node, true)
		return
	}
	target := p.sink.NewLeaf(KindIdentifier, p.advance())
	p.sink.SetField(node, fieldTarget, target)
	p.sink.AddChild(node, target)
}

func (p *parser[N, S]) parseExpressionStatement() N {
	expr := p.parseExpression()
	node := p.sink.Store(Node{Kind: KindExpressionStatement, Start: p.sink.Start(expr), Leading: p.sink.Leading(expr)})
	p.sink.SetField(node, fieldExpression, expr)
	p.sink.AddChild(node, expr)
	if p.at(token.Hash) && p.iteratorCallExpression(expr) {
		switch p.peekDirectiveKeyword() {
		case dirElseif, dirElse, dirEndif:
			p.sink.SetMissingSemi(node, true)
			return node
		}
	}
	p.consumeTrailingSemi(node)
	return node
}

func (p *parser[N, S]) iteratorCallExpression(expr N) bool {
	if expr == p.sink.Nil() || p.sink.Kind(expr) != KindCallExpression {
		return false
	}
	args := p.sink.Field(expr, fieldArguments)
	if args == p.sink.Nil() {
		return false
	}
	for _, child := range p.sink.Children(args) {
		if p.sink.Kind(child) == KindIteratorArgument {
			return true
		}
	}
	return false
}
