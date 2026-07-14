package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser) parseStatement() *Node {
	if !p.enterDepth() {
		defer p.exitDepth()
		p.broken = true
		tok := p.cur()
		return &Node{Kind: KindEmptyStatement, Start: tok.Start.Offset, End: tok.Start.Offset, HasError: true}
	}
	defer p.exitDepth()

	if isKeywordToken(p.cur().Kind) && p.peek(1).Kind == token.LParen && !nativeParenthesizedStatement(p.cur().Kind) {
		return p.parseExpressionStatement()
	}

	switch p.cur().Kind {
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
		return &Node{Kind: KindEmptyStatement, Tok: tok, Start: tok.Start.Offset, End: tok.End.Offset, Leading: tok.LeadingTrivia, Trailing: tok.TrailingTrivia}
	case token.Hash:
		if dk := p.peekDirectiveKeyword(); dk == dirElseif || dk == dirElse || dk == dirEndif {
			return &Node{Kind: KindEmptyStatement, Start: p.cur().Start.Offset, End: p.cur().Start.Offset}
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

func isLabelStart(p *parser) bool {
	return p.cur().Kind == token.Identifier && p.peek(1).Kind == token.Colon && p.peek(2).Kind != token.Colon
}

func (p *parser) macroFunctionDefinitionStart() bool {
	if !p.macroFunctionQualifierStart() {
		return false
	}
	depth := 0
	foundParams := false
	for i := 1; ; i++ {
		switch p.peek(i).Kind {
		case token.LParen:
			depth++
			foundParams = true
		case token.RParen:
			depth--
			if foundParams && depth == 0 {
				return p.peek(i+1).Kind == token.LBrace
			}
		case token.EOF, token.Semicolon:
			return false
		default:
		}
	}
}

func isMacroInvocationBlockStart(p *parser) bool {
	if p.cur().Kind != token.Identifier || p.peek(1).Kind != token.LParen {
		return false
	}
	depth := 0
	iteratorArgument := false
	for i := 1; ; i++ {
		tk := p.peek(i).Kind
		if tk == token.EOF {
			return false
		}
		switch {
		case tk == token.LParen:
			depth++
		case depth == 1 && tk == token.KwNew && p.peek(i+2).Kind == token.Colon:
			iteratorArgument = true
		case tk == token.RParen:
			depth--
			if depth == 0 {
				next := p.peek(i + 1).Kind
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

func (p *parser) parseMacroInvocationBlock() *Node {
	nameTok := p.advance()
	name := p.newLeaf(KindIdentifier, nameTok)
	args := p.parseArgumentList()
	body := p.parseControlledStatement()
	node := p.newNode(KindMacroInvocationBlock, name, args, body)
	setField(node, "function", name)
	setField(node, "arguments", args)
	setField(node, "body", body)
	return node
}

func (p *parser) parseBlock() *Node {
	lb := p.advance() // '{'
	node := &Node{Kind: KindBlock, Start: lb.Start.Offset, Leading: lb.LeadingTrivia}
	items := p.parseItemSequence(itemGrammar{
		parseItem:        func(p *parser) *Node { return p.parseStatement() },
		recoveryContext:  "statement",
		recoveryExpected: []token.Kind{token.Semicolon, token.RBrace},
		stop: func(p *parser) bool {
			p.abortIfSharedAcrossBranch()
			return p.at(token.RBrace)
		},
	})
	for _, it := range items {
		node.addChild(it)
	}
	if p.at(token.RBrace) {
		rb := p.advance()
		node.End = rb.End.Offset
		node.Trailing = rb.TrailingTrivia
	} else {
		node.HasError = true
		node.End = p.cur().Start.Offset
	}
	return node
}

func (p *parser) parseControlledStatement() *Node {
	return p.parseStatement()
}

func (p *parser) consumeTrailingSemi(node *Node) {
	switch {
	case p.at(token.Semicolon):
		semi := p.advance()
		node.End = semi.End.Offset
		node.Trailing = semi.TrailingTrivia
	case p.missingSemiOK():
		node.MissingSemi = true
	default:
		node.HasError = true
	}
}

func (p *parser) parseIfStatement() *Node {
	kw := p.advance()
	condition := p.parseParenCondition()
	consequence := p.parseControlledStatement()
	node := p.newNode(KindIfStatement, condition, consequence)
	setField(node, "condition", condition)
	setField(node, "consequence", consequence)
	node.Tok = kw
	node.Start = kw.Start.Offset
	node.Leading = kw.LeadingTrivia
	if p.at(token.KwElse) {
		p.advance()
		alternative := p.parseControlledStatement()
		setField(node, "alternative", alternative)
		node.Children = append(node.Children, alternative)
		node.End = alternative.End
		node.Trailing = alternative.Trailing
		if alternative.HasError {
			node.HasError = true
		}
	}
	return node
}

func (p *parser) parseParenCondition() *Node {
	if !p.at(token.LParen) {
		n := &Node{Kind: KindParenthesizedExpression, HasError: true}
		return n
	}
	return p.parseParenthesized()
}

func (p *parser) parseWhileStatement() *Node {
	kw := p.advance()
	condition := p.parseParenCondition()
	body := p.parseControlledStatement()
	node := p.newNode(KindWhileStatement, condition, body)
	setField(node, "condition", condition)
	setField(node, "body", body)
	node.Tok = kw
	node.Start = kw.Start.Offset
	node.Leading = kw.LeadingTrivia
	return node
}

func (p *parser) parseDoWhileStatement() *Node {
	kw := p.advance()
	body := p.parseControlledStatement()
	node := &Node{Kind: KindDoWhileStatement, Tok: kw, Start: kw.Start.Offset, Leading: kw.LeadingTrivia}
	setField(node, "body", body)
	node.addChild(body)
	if p.at(token.KwWhile) {
		p.advance()
	} else {
		node.HasError = true
	}
	condition := p.parseParenCondition()
	setField(node, "condition", condition)
	node.addChild(condition)
	p.consumeTrailingSemi(node)
	return node
}

func (p *parser) parseForInit() *Node {
	if !p.at(token.KwNew) && !p.at(token.KwStatic) {
		return p.parseExpression()
	}
	kw := p.advance()
	node := &Node{Kind: KindVariableDeclaration, Start: kw.Start.Offset, Leading: kw.LeadingTrivia}
	node.addChild(p.newLeaf(KindIdentifier, kw))
	for _, d := range p.parseDeclaratorList() {
		node.addChild(d)
	}
	return node
}

func (p *parser) parseForStatement() *Node {
	kw := p.advance()
	node := &Node{Kind: KindForStatement, Tok: kw, Start: kw.Start.Offset, Leading: kw.LeadingTrivia}
	if !p.at(token.LParen) {
		node.HasError = true
		return node
	}
	p.advance()

	if !p.at(token.Semicolon) {
		init := p.parseForInit()
		setField(node, "init", init)
		node.addChild(init)
	}
	if p.at(token.Semicolon) {
		p.advance()
	} else {
		node.HasError = true
	}

	if !p.at(token.Semicolon) {
		cond := p.parseExpression()
		setField(node, "condition", cond)
		node.addChild(cond)
	}
	if p.at(token.Semicolon) {
		p.advance()
	} else {
		node.HasError = true
	}

	if !p.at(token.RParen) {
		update := p.parseExpression()
		setField(node, "increment", update)
		node.addChild(update)
	}
	if p.at(token.RParen) {
		p.advance()
	} else {
		node.HasError = true
	}

	body := p.parseControlledStatement()
	setField(node, "body", body)
	node.addChild(body)
	return node
}
