package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser[N, S]) parsePostfix() N {
	expr := p.parsePrimary()
	for {
		switch p.curKind() {
		case token.LParen:
			expr = p.parseCall(expr)
		case token.LBracket:
			expr = p.parseSubscript(expr)
		case token.LBrace:
			expr = p.parseCellSelection(expr)
		case token.Dot, token.ColonColon:
			expr = p.parseMemberSelection(expr)
		case token.PlusPlus, token.MinusMinus:
			opTok := p.advance()
			node := p.sink.NewNode(KindUpdateExpression, expr)
			p.sink.SetField(node, fieldExpression, expr)
			p.sink.SetToken(node, opTok)
			expr = node
		case token.Identifier:
			if p.parsingDimension && p.cur().Text(p.source) == "char" {
				return expr
			}
			opTok := p.advance()
			node := p.sink.NewNode(KindUnaryExpression, expr)
			p.sink.SetField(node, fieldExpression, expr)
			p.sink.SetToken(node, opTok)
			p.sink.SetEnd(node, opTok.End.Offset)
			p.sink.SetTrailing(node, opTok.TrailingTrivia)
			expr = node
		case token.Lt:
			if p.sink.End(expr) != p.cur().Start.Offset || !p.hasAngleClose(p.pos) {
				return expr
			}
			expr = p.parseMacroPostfixSelection(expr)
		default:
			return expr
		}
	}
}

func (p *parser[N, S]) parseMacroPostfixSelection(target N) N {
	lt := p.advance()
	children := []N{target}
	depth := 1
	last := lt
	for !p.atEnd() && depth > 0 {
		tok := p.advance()
		last = tok
		switch tok.Kind {
		case token.Lt:
			depth++
		case token.Gt:
			depth--
		case token.Identifier, token.MacroParam:
			children = p.sink.Append(children, p.sink.NewLeaf(KindIdentifier, tok))
		case token.IntLiteral, token.FloatLiteral, token.CharLiteral, token.StringLiteral, token.PackedString:
			children = p.sink.Append(children, p.sink.NewLeaf(KindLiteral, tok))
		default:
		}
	}
	node := p.directiveSpan(KindMacroBody, p.sink.Start(target), last.End.Offset, p.sink.Leading(target), last.TrailingTrivia)
	p.sink.SetToken(node, lt)
	p.sink.SetChildren(node, children)
	p.sink.SetField(node, fieldTarget, target)
	return node
}

func (p *parser[N, S]) parseCellSelection(target N) N {
	open := p.advance()
	index := p.parseExpression()
	node := p.sink.NewNode(KindSubscriptExpression, target, index)
	p.sink.SetToken(node, open)
	p.sink.SetField(node, fieldArray, target)
	p.sink.SetField(node, fieldIndex, index)
	if p.at(token.RBrace) {
		rb := p.advance()
		p.sink.SetEnd(node, rb.End.Offset)
		p.sink.SetTrailing(node, rb.TrailingTrivia)
	} else {
		p.sink.SetHasError(node, true)
		p.emitMissingToken(token.RBrace, "cell selection")
	}
	return node
}

func (p *parser[N, S]) parseMemberSelection(target N) N {
	op := p.advance()
	if !p.at(token.Identifier) && !p.at(token.MacroParam) && !isKeywordToken(p.curKind()) {
		node := p.sink.NewNode(KindBinaryExpression, target)
		p.sink.SetToken(node, op)
		p.sink.SetEnd(node, op.End.Offset)
		p.sink.SetTrailing(node, op.TrailingTrivia)
		p.sink.SetHasError(node, true)
		p.sink.SetErrorMessage(node, "expected identifier after "+op.Kind.String())
		p.sink.SetErrorOffset(node, p.cur().Start.Offset)
		p.sink.SetErrorFound(node, p.curKind())
		p.sink.SetErrorExpected(node, []token.Kind{token.Identifier})
		p.emitMissing(DiagnosticMissingIdentifier, p.sink.ErrorMessage(node), token.Identifier)
		return node
	}
	member := p.sink.NewLeaf(KindIdentifier, p.advance())
	node := p.sink.NewNode(KindBinaryExpression, target, member)
	p.sink.SetField(node, fieldLeft, target)
	p.sink.SetField(node, fieldRight, member)
	p.sink.SetToken(node, op)
	return node
}

func (p *parser[N, S]) parseCall(callee N) N {
	args := p.parseArgumentList()
	node := p.sink.NewNode(KindCallExpression, callee, args)
	p.sink.SetField(node, fieldFunction, callee)
	p.sink.SetField(node, fieldArguments, args)
	return node
}

func (p *parser[N, S]) parseCallArgument() N {
	if p.at(token.KwNew) && p.peekKind(1) == token.Identifier &&
		p.peekKind(2) == token.Colon && p.peekKind(3) == token.Identifier &&
		(p.peekKind(4) == token.Comma || p.peekKind(4) == token.RParen) {
		first := p.advance()
		p.advance()
		p.advance()
		last := p.advance()
		return p.sink.Store(Node{
			Kind:     KindIteratorArgument,
			Start:    first.Start.Offset,
			End:      last.End.Offset,
			Leading:  first.LeadingTrivia,
			Trailing: last.TrailingTrivia,
		})
	}
	if !p.at(token.Dot) {
		return p.parseAssignment()
	}
	dot := p.advance()
	if !p.at(token.Identifier) {
		n := p.sink.NewLeaf(KindRaw, dot)
		p.sink.SetHasError(n, true)
		return n
	}
	nameTok := p.advance()
	name := p.sink.Store(Node{Kind: KindArgumentName, Tok: nameTok, Start: dot.Start.Offset, End: nameTok.End.Offset, Leading: dot.LeadingTrivia, Trailing: nameTok.TrailingTrivia})
	if !p.at(token.Assign) {
		p.sink.SetHasError(name, true)
		return name
	}
	opTok := p.advance()
	right := p.parseAssignment()
	node := p.sink.NewNode(KindAssignmentExpression, name, right)
	p.sink.SetField(node, fieldLeft, name)
	p.sink.SetField(node, fieldRight, right)
	p.sink.SetToken(node, opTok)
	return node
}

func (p *parser[N, S]) parseArgumentList() N {
	lp := p.advance()
	node := p.sink.Store(Node{Kind: KindArgumentList, Start: lp.Start.Offset, Leading: lp.LeadingTrivia})
	for !p.atEnd() && !p.at(token.RParen) {
		startPos := p.pos
		endPos := p.argumentEnd(startPos)
		wasBroken := p.broken
		diagnosticStart := len(p.diagnostics)
		arg := p.parseCallArgument()
		if arg == p.sink.Nil() || p.sink.HasError(arg) || p.broken || p.pos != endPos {
			p.pos = startPos
			p.broken = wasBroken
			p.diagnostics = p.diagnostics[:diagnosticStart]
			arg = p.consumeStructuredMacroArgument(endPos)
		}
		p.sink.AddChild(node, arg)
		if p.at(token.Comma) {
			comma := p.advance()
			p.mergeCommaTrivia(arg, comma)
		}
	}
	if p.at(token.RParen) {
		rp := p.advance()
		p.sink.SetEnd(node, rp.End.Offset)
		p.sink.SetTrailing(node, rp.TrailingTrivia)
	} else {
		p.sink.SetHasError(node, true)
		p.emitMissingToken(token.RParen, "argument list")
	}
	return node
}

func (p *parser[N, S]) argumentEnd(start int) int {
	parenDepth, bracketDepth, braceDepth, angleDepth := 0, 0, 0, 0
	for i := start; i < p.toks.len(); i++ {
		kind := p.toks.kind(i)
		if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 && angleDepth == 0 &&
			(kind == token.Comma || kind == token.RParen || kind == token.EOF) {
			return i
		}
		switch kind {
		case token.LParen:
			parenDepth++
		case token.RParen:
			parenDepth--
		case token.LBracket:
			bracketDepth++
		case token.RBracket:
			bracketDepth--
		case token.LBrace:
			braceDepth++
		case token.RBrace:
			braceDepth--
		case token.Lt:
			if p.hasAngleClose(i) {
				angleDepth++
			}
		case token.Gt:
			if angleDepth > 0 {
				angleDepth--
			}
		default:
		}
	}
	return p.toks.len() - 1
}

func (p *parser[N, S]) hasAngleClose(start int) bool {
	if !p.angleCloseBuilt {
		p.buildAngleClose()
	}
	return start >= 0 && start < len(p.angleClose) && p.angleClose[start]
}

func (p *parser[N, S]) buildAngleClose() {
	p.angleClose = make([]bool, p.toks.len())
	stack := make([]int, 0, 8)
	for i := range p.toks.len() {
		switch p.toks.kind(i) {
		case token.Lt:
			stack = append(stack, i)
		case token.Gt:
			if len(stack) != 0 {
				last := len(stack) - 1
				p.angleClose[stack[last]] = true
				stack = stack[:last]
			}
		case token.RParen, token.Semicolon, token.EOF:
			stack = stack[:0]
		default:
		}
	}
	p.angleCloseBuilt = true
}

func (p *parser[N, S]) consumeStructuredMacroArgument(endPos int) N {
	start := p.cur()
	last := start
	var parts []N
	for p.pos < endPos {
		kind := p.curKind()
		last = p.advance()
		switch {
		case kind == token.Identifier || kind == token.MacroParam || isKeywordToken(kind):
			parts = p.sink.Append(parts, p.sink.NewLeaf(KindIdentifier, last))
		case isLiteralToken(kind):
			parts = p.sink.Append(parts, p.sink.NewLeaf(KindLiteral, last))
		}
	}
	node := p.directiveSpan(KindMacroBody, start.Start.Offset, last.End.Offset, start.LeadingTrivia, last.TrailingTrivia)
	p.sink.SetChildren(node, parts)
	return node
}

func isLiteralToken(kind token.Kind) bool {
	switch kind {
	case token.IntLiteral, token.FloatLiteral, token.CharLiteral, token.StringLiteral, token.PackedString:
		return true
	default:
		return false
	}
}

func (p *parser[N, S]) parseSubscript(target N) N {
	open := p.advance()
	var index N
	if !p.at(token.RBracket) {
		index = p.parseExpression()
	}
	node := p.sink.NewNode(KindSubscriptExpression, target, index)
	p.sink.SetToken(node, open)
	p.sink.SetField(node, fieldArray, target)
	p.sink.SetField(node, fieldIndex, index)
	if p.at(token.RBracket) {
		rb := p.advance()
		p.sink.SetEnd(node, rb.End.Offset)
		p.sink.SetTrailing(node, rb.TrailingTrivia)
	} else {
		p.sink.SetHasError(node, true)
		p.emitMissingToken(token.RBracket, "subscript")
	}
	return node
}
