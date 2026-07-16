package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser[N, S]) parseStatement() N {
	if !p.enterDepth() {
		defer p.exitDepth()
		p.broken = true
		tok := p.cur()
		return p.sink.Store(Node{Kind: KindEmptyStatement, Start: tok.Start.Offset, End: tok.Start.Offset, HasError: true})
	}
	defer p.exitDepth()

	if isKeywordToken(p.curKind()) && p.peekKind(1) == token.LParen && !nativeParenthesizedStatement(p.curKind()) {
		return p.parseExpressionStatement()
	}

	switch p.curKind() {
	case token.LBrace:
		return p.parseBlock()
	case token.KwIf:
		return p.parseIfStatement()
	case token.KwWhile:
		return p.parseWhileStatement()
	case token.KwDo:
		return p.parseDoWhileStatement()
	case token.KwFor:
		return p.parseForStatement()
	case token.KwSwitch:
		return p.parseSwitchStatement()
	case token.KwReturn:
		return p.parseReturnStatement()
	case token.KwBreak:
		return p.simpleKeywordStatement(KindBreakStatement)
	case token.KwContinue:
		return p.simpleKeywordStatement(KindContinueStatement)
	case token.KwGoto:
		return p.parseGotoStatement()
	case token.KwState:
		return p.parseStateStatement()
	case token.KwNew, token.KwStatic, token.KwConst:
		return p.parseVariableDeclaration()
	case token.Semicolon:
		tok := p.advance()
		return p.sink.Store(Node{Kind: KindEmptyStatement, Tok: tok, Start: tok.Start.Offset, End: tok.End.Offset, Leading: tok.LeadingTrivia, Trailing: tok.TrailingTrivia})
	case token.Hash:
		if dk := p.peekDirectiveKeyword(); dk == dirElseif || dk == dirElse || dk == dirEndif {
			return p.sink.Store(Node{Kind: KindEmptyStatement, Start: p.cur().Start.Offset, End: p.cur().Start.Offset})
		}
		return p.parseSingleDirective()
	default:
		if p.macroFunctionDefinitionStart() {
			return p.parseFunctionLike(p.collectQualifiers())
		}
		if isLabelStart(p) {
			return p.parseLabelStatement()
		}
		if isMacroInvocationBlockStart(p) {
			return p.parseMacroInvocationBlock()
		}
		return p.parseExpressionStatement()
	}
}

func nativeParenthesizedStatement(kind token.Kind) bool {
	switch kind {
	case token.KwIf, token.KwWhile, token.KwFor, token.KwSwitch, token.KwReturn,
		token.KwNew, token.KwStatic, token.KwConst:
		return true
	default:
		return false
	}
}

func isLabelStart[N comparable, S nodeSink[N]](p *parser[N, S]) bool {
	return p.curKind() == token.Identifier && p.peekKind(1) == token.Colon && p.peekKind(2) != token.Colon
}

func (p *parser[N, S]) macroFunctionDefinitionStart() bool {
	if !p.macroFunctionQualifierStart() {
		return false
	}
	depth := 0
	foundParams := false
	for i := 1; ; i++ {
		switch p.peekKind(i) {
		case token.LParen:
			depth++
			foundParams = true
		case token.RParen:
			depth--
			if foundParams && depth == 0 {
				return p.peekKind(i+1) == token.LBrace
			}
		case token.EOF, token.Semicolon:
			return false
		default:
		}
	}
}

func isMacroInvocationBlockStart[N comparable, S nodeSink[N]](p *parser[N, S]) bool {
	if p.curKind() != token.Identifier || p.peekKind(1) != token.LParen {
		return false
	}
	depth := 0
	iteratorArgument := false
	for i := 1; ; i++ {
		tk := p.peekKind(i)
		if tk == token.EOF {
			return false
		}
		switch {
		case tk == token.LParen:
			depth++
		case depth == 1 && tk == token.KwNew && p.peekKind(i+2) == token.Colon:
			iteratorArgument = true
		case tk == token.RParen:
			depth--
			if depth == 0 {
				next := p.peekKind(i + 1)
				return canStartMacroControlledStatement(next) || iteratorArgument && next != token.Hash && next != token.Semicolon
			}
		}
	}
}

func canStartMacroControlledStatement(kind token.Kind) bool {
	switch kind {
	case token.LBrace, token.KwIf, token.KwWhile, token.KwDo, token.KwFor, token.KwSwitch:
		return true
	default:
		return false
	}
}

func (p *parser[N, S]) parseMacroInvocationBlock() N {
	nameTok := p.advance()
	name := p.sink.NewLeaf(KindIdentifier, nameTok)
	args := p.parseArgumentList()
	body := p.parseControlledStatement()
	node := p.sink.NewNode(KindMacroInvocationBlock, name, args, body)
	p.sink.SetField(node, fieldFunction, name)
	p.sink.SetField(node, fieldArguments, args)
	p.sink.SetField(node, fieldBody, body)
	return node
}

func (p *parser[N, S]) parseBlock() N {
	lb := p.advance() // '{'
	node := p.sink.Store(Node{Kind: KindBlock, Start: lb.Start.Offset, Leading: lb.LeadingTrivia})
	items := p.parseItemSequence(itemGrammar[N, S]{
		parseMode:        itemParseStatement,
		recoveryContext:  "statement",
		recoveryExpected: []token.Kind{token.Semicolon, token.RBrace},
		stopKind:         token.RBrace,
		abortAtStop:      true,
	})
	for _, it := range items {
		p.sink.AddChild(node, it)
	}
	if p.at(token.RBrace) {
		rb := p.advance()
		p.sink.SetEnd(node, rb.End.Offset)
		p.sink.SetTrailing(node, rb.TrailingTrivia)
	} else {
		p.sink.SetHasError(node, true)
		p.emitMissingToken(token.RBrace, "block")
		p.sink.SetEnd(node, p.cur().Start.Offset)
	}
	return node
}

func (p *parser[N, S]) parseControlledStatement() N {
	return p.parseStatement()
}

func (p *parser[N, S]) consumeTrailingSemi(node N) {
	switch {
	case p.at(token.Semicolon):
		semi := p.advance()
		p.sink.SetEnd(node, semi.End.Offset)
		p.sink.SetTrailing(node, semi.TrailingTrivia)
	case p.missingSemiOK():
		p.sink.SetMissingSemi(node, true)
	default:
		p.sink.SetHasError(node, true)
	}
}

func (p *parser[N, S]) parseIfStatement() N {
	kw := p.advance()
	condition := p.parseParenCondition()
	consequence := p.parseControlledStatement()
	node := p.sink.NewNode(KindIfStatement, condition, consequence)
	p.sink.SetField(node, fieldCondition, condition)
	p.sink.SetField(node, fieldConsequence, consequence)
	p.sink.SetToken(node, kw)
	p.sink.SetStart(node, kw.Start.Offset)
	p.sink.SetLeading(node, kw.LeadingTrivia)
	if p.at(token.KwElse) {
		p.advance()
		alternative := p.parseControlledStatement()
		p.sink.SetField(node, fieldAlternative, alternative)
		p.sink.AddChild(node, alternative)
		p.sink.SetEnd(node, p.sink.End(alternative))
		p.sink.SetTrailing(node, p.sink.Trailing(alternative))
		if p.sink.HasError(alternative) {
			p.sink.SetHasError(node, true)
		}
	}
	return node
}

func (p *parser[N, S]) parseParenCondition() N {
	if !p.at(token.LParen) {
		p.emitMissingToken(token.LParen, "condition")
		n := p.sink.Store(Node{Kind: KindParenthesizedExpression, HasError: true})
		return n
	}
	return p.parseParenthesized()
}

func (p *parser[N, S]) parseWhileStatement() N {
	kw := p.advance()
	condition := p.parseParenCondition()
	body := p.parseControlledStatement()
	node := p.sink.NewNode(KindWhileStatement, condition, body)
	p.sink.SetField(node, fieldCondition, condition)
	p.sink.SetField(node, fieldBody, body)
	p.sink.SetToken(node, kw)
	p.sink.SetStart(node, kw.Start.Offset)
	p.sink.SetLeading(node, kw.LeadingTrivia)
	return node
}

func (p *parser[N, S]) parseDoWhileStatement() N {
	kw := p.advance()
	body := p.parseControlledStatement()
	node := p.sink.Store(Node{Kind: KindDoWhileStatement, Tok: kw, Start: kw.Start.Offset, Leading: kw.LeadingTrivia})
	p.sink.SetField(node, fieldBody, body)
	p.sink.AddChild(node, body)
	if p.at(token.KwWhile) {
		p.advance()
	} else {
		p.sink.SetHasError(node, true)
	}
	condition := p.parseParenCondition()
	p.sink.SetField(node, fieldCondition, condition)
	p.sink.AddChild(node, condition)
	p.consumeTrailingSemi(node)
	return node
}

func (p *parser[N, S]) parseForInit() N {
	if !p.at(token.KwNew) && !p.at(token.KwStatic) {
		return p.parseExpression()
	}
	kw := p.advance()
	node := p.sink.Store(Node{Kind: KindVariableDeclaration, Start: kw.Start.Offset, Leading: kw.LeadingTrivia})
	p.sink.AddChild(node, p.sink.NewLeaf(KindIdentifier, kw))
	for _, d := range p.parseDeclaratorList() {
		p.sink.AddChild(node, d)
	}
	return node
}

func (p *parser[N, S]) parseForStatement() N {
	kw := p.advance()
	node := p.sink.Store(Node{Kind: KindForStatement, Tok: kw, Start: kw.Start.Offset, Leading: kw.LeadingTrivia})
	if !p.at(token.LParen) {
		p.sink.SetHasError(node, true)
		return node
	}
	p.advance()

	if !p.at(token.Semicolon) {
		init := p.parseForInit()
		p.sink.SetField(node, fieldInit, init)
		p.sink.AddChild(node, init)
	}
	if p.at(token.Semicolon) {
		p.advance()
	} else {
		p.sink.SetHasError(node, true)
	}

	if !p.at(token.Semicolon) {
		cond := p.parseExpression()
		p.sink.SetField(node, fieldCondition, cond)
		p.sink.AddChild(node, cond)
	}
	if p.at(token.Semicolon) {
		p.advance()
	} else {
		p.sink.SetHasError(node, true)
	}

	if !p.at(token.RParen) {
		update := p.parseExpression()
		p.sink.SetField(node, fieldIncrement, update)
		p.sink.AddChild(node, update)
	}
	if p.at(token.RParen) {
		p.advance()
	} else {
		p.sink.SetHasError(node, true)
	}

	body := p.parseControlledStatement()
	p.sink.SetField(node, fieldBody, body)
	p.sink.AddChild(node, body)
	return node
}
