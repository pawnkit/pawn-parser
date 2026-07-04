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
	lp := p.advance() // '('
	return p.parseBracketedList(KindArgumentList, lp, token.RParen, (*parser).parseCallArgument)
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
