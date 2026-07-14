package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser) parsePostfix() *Node {
	expr := p.parsePrimary()
	for {
		switch p.cur().Kind {
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
			node := p.newNode(KindUpdateExpression, expr)
			setField(node, "expression", expr)
			node.Tok = opTok
			expr = node
		default:
			return expr
		}
	}
}

func (p *parser) parseCellSelection(target *Node) *Node {
	p.advance()
	index := p.parseExpression()
	node := p.newNode(KindSubscriptExpression, target, index)
	setField(node, "array", target)
	setField(node, "index", index)
	if p.at(token.RBrace) {
		rb := p.advance()
		node.End = rb.End.Offset
		node.Trailing = rb.TrailingTrivia
	} else {
		node.HasError = true
	}
	return node
}

func (p *parser) parseMemberSelection(target *Node) *Node {
	op := p.advance()
	if !p.at(token.Identifier) && !p.at(token.MacroParam) {
		node := p.newNode(KindBinaryExpression, target)
		node.Tok = op
		node.HasError = true
		return node
	}
	member := p.newLeaf(KindIdentifier, p.advance())
	node := p.newNode(KindBinaryExpression, target, member)
	setField(node, "left", target)
	setField(node, "right", member)
	node.Tok = op
	return node
}

func (p *parser) parseCall(callee *Node) *Node {
	args := p.parseArgumentList()
	node := p.newNode(KindCallExpression, callee, args)
	setField(node, "function", callee)
	setField(node, "arguments", args)
	return node
}

func (p *parser) parseCallArgument() *Node {
	if p.at(token.KwNew) && p.peek(1).Kind == token.Identifier &&
		p.peek(2).Kind == token.Colon && p.peek(3).Kind == token.Identifier &&
		(p.peek(4).Kind == token.Comma || p.peek(4).Kind == token.RParen) {
		first := p.advance()
		p.advance()
		p.advance()
		last := p.advance()
		return &Node{
			Kind:     KindIteratorArgument,
			Start:    first.Start.Offset,
			End:      last.End.Offset,
			Leading:  first.LeadingTrivia,
			Trailing: last.TrailingTrivia,
		}
	}
	if !p.at(token.Dot) {
		return p.parseAssignment()
	}
	dot := p.advance()
	if !p.at(token.Identifier) {
		n := p.newLeaf(KindRaw, dot)
		n.HasError = true
		return n
	}
	nameTok := p.advance()
	name := &Node{Kind: KindArgumentName, Tok: nameTok, Start: dot.Start.Offset, End: nameTok.End.Offset, Leading: dot.LeadingTrivia, Trailing: nameTok.TrailingTrivia}
	if !p.at(token.Assign) {
		name.HasError = true
		return name
	}
	opTok := p.advance()
	right := p.parseAssignment()
	node := p.newNode(KindAssignmentExpression, name, right)
	setField(node, "left", name)
	setField(node, "right", right)
	node.Tok = opTok
	return node
}

func (p *parser) parseArgumentList() *Node {
	lp := p.advance()
	node := &Node{Kind: KindArgumentList, Start: lp.Start.Offset, Leading: lp.LeadingTrivia}
	for !p.atEnd() && !p.at(token.RParen) {
		startPos := p.pos
		endPos := p.argumentEnd(startPos)
		wasBroken := p.broken
		arg := p.parseCallArgument()
		if arg == nil || arg.HasError || p.broken || p.pos != endPos {
			p.pos = startPos
			p.broken = wasBroken
			arg = p.consumeStructuredMacroArgument(endPos)
		}
		node.addChild(arg)
		if p.at(token.Comma) {
			comma := p.advance()
			mergeCommaTrivia(arg, comma)
		}
	}
	if p.at(token.RParen) {
		rp := p.advance()
		node.End = rp.End.Offset
		node.Trailing = rp.TrailingTrivia
	} else {
		node.HasError = true
	}
	return node
}

func (p *parser) argumentEnd(start int) int {
	parenDepth, bracketDepth, braceDepth, angleDepth := 0, 0, 0, 0
	for i := start; i < len(p.toks); i++ {
		kind := p.toks[i].Kind
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
	return len(p.toks) - 1
}

func (p *parser) hasAngleClose(start int) bool {
	depth := 1
	for i := start + 1; i < len(p.toks); i++ {
		switch p.toks[i].Kind {
		case token.Lt:
			depth++
		case token.Gt:
			depth--
			if depth == 0 {
				return true
			}
		case token.RParen, token.Semicolon, token.EOF:
			return false
		default:
		}
	}
	return false
}

func (p *parser) consumeStructuredMacroArgument(endPos int) *Node {
	start := p.cur()
	last := start
	var parts []*Node
	for p.pos < endPos {
		kind := p.cur().Kind
		last = p.advance()
		switch {
		case kind == token.Identifier || kind == token.MacroParam || isKeywordToken(kind):
			parts = append(parts, p.newLeaf(KindIdentifier, last))
		case isLiteralToken(kind):
			parts = append(parts, p.newLeaf(KindLiteral, last))
		}
	}
	node := directiveSpan(p.source, KindMacroBody, start.Start.Offset, last.End.Offset, start.LeadingTrivia, last.TrailingTrivia)
	node.Children = parts
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

func (p *parser) parseSubscript(target *Node) *Node {
	p.advance()
	var index *Node
	if !p.at(token.RBracket) {
		index = p.parseExpression()
	}
	node := p.newNode(KindSubscriptExpression, target, index)
	setField(node, "array", target)
	setField(node, "index", index)
	if p.at(token.RBracket) {
		rb := p.advance()
		node.End = rb.End.Offset
		node.Trailing = rb.TrailingTrivia
	} else {
		node.HasError = true
	}
	return node
}
