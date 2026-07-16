package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser[N, S]) parsePrimary() N {
	tok := p.cur()
	if isKeywordToken(tok.Kind) && p.peekKind(1) == token.LParen {
		p.advance()
		return p.sink.NewLeaf(KindIdentifier, tok)
	}
	switch tok.Kind {
	case token.Identifier, token.MacroParam:
		p.advance()
		return p.sink.NewLeaf(KindIdentifier, tok)
	case token.IntLiteral, token.FloatLiteral, token.CharLiteral, token.KwNull:
		p.advance()
		return p.sink.NewLeaf(KindLiteral, tok)
	case token.StringLiteral, token.PackedString:
		return p.parseStringConcat()
	case token.Hash:
		if p.peekKind(1) == token.Identifier {
			return p.parseStringConcat()
		}
		p.advance()
		n := p.sink.NewLeaf(KindLiteral, tok)
		p.sink.SetHasError(n, true)
		return n
	case token.Ellipsis:
		p.advance()
		return p.sink.NewLeaf(KindLiteral, tok)
	case token.LParen:
		return p.parseParenthesized()
	case token.LBrace:
		return p.parseArrayLiteral()
	default:
		if isExpressionBoundary(tok.Kind) {
			p.emitMissing(DiagnosticMissingExpression, "expected expression",
				token.Identifier, token.IntLiteral, token.LParen)
			n := p.sink.Store(Node{
				Kind: KindLiteral, Tok: tok, Start: tok.Start.Offset, End: tok.Start.Offset, HasError: true,
				Leading: tok.LeadingTrivia,
			})
			return n
		}
		p.advance()
		n := p.sink.NewLeaf(KindLiteral, tok)
		p.sink.SetHasError(n, true)
		p.emitDiagnostic(Diagnostic{
			Code:    DiagnosticUnexpectedToken,
			Message: "unexpected token in expression", Range: tokenRange(tok), Found: tok,
			Recovery: Recovery{Kind: RecoveryRemove, Range: tokenRange(tok), Confidence: RecoveryExact},
		})
		return n
	}
}

func isExpressionBoundary(kind token.Kind) bool {
	switch kind {
	case token.EOF, token.RParen, token.RBracket, token.RBrace, token.Comma, token.Semicolon:
		return true
	default:
		return false
	}
}

func (p *parser[N, S]) isStringPartStart() bool {
	if p.at(token.StringLiteral) || p.at(token.PackedString) {
		return true
	}
	if p.at(token.Identifier) || p.at(token.MacroParam) || isKeywordToken(p.curKind()) {
		return true
	}
	return p.at(token.Hash) && p.peekKind(1) == token.Identifier
}

func (p *parser[N, S]) parseStringPart() N {
	if p.at(token.Hash) {
		return p.parseStringizeExpression()
	}
	tok := p.advance()
	if tok.Kind == token.Identifier || tok.Kind == token.MacroParam || isKeywordToken(tok.Kind) {
		return p.sink.NewLeaf(KindIdentifier, tok)
	}
	return p.sink.NewLeaf(KindLiteral, tok)
}

func (p *parser[N, S]) parseStringizeExpression() N {
	hash := p.advance() // '#'
	nameTok := p.advance()
	name := p.sink.NewLeaf(KindIdentifier, nameTok)
	node := p.sink.Store(Node{Kind: KindStringizeExpression, Tok: hash, Start: hash.Start.Offset, End: nameTok.End.Offset, Leading: hash.LeadingTrivia, Trailing: nameTok.TrailingTrivia})
	p.sink.SetField(node, fieldName, name)
	p.sink.AddChild(node, name)
	return node
}

func (p *parser[N, S]) parseStringConcat() N {
	first := p.parseStringPart()
	if !p.isStringPartStart() {
		return first
	}
	concat := p.sink.Store(Node{Kind: KindStringConcat, Start: p.sink.Start(first), Leading: p.sink.Leading(first)})
	p.sink.AddChild(concat, first)
	for p.isStringPartStart() {
		p.sink.AddChild(concat, p.parseStringPart())
	}
	return concat
}

func (p *parser[N, S]) parseParenthesized() N {
	lp := p.advance()
	inner := p.parseExpression()
	node := p.sink.NewNode(KindParenthesizedExpression, inner)
	p.sink.SetField(node, fieldExpression, inner)
	p.sink.SetStart(node, lp.Start.Offset)
	p.sink.SetLeading(node, lp.LeadingTrivia)
	if p.at(token.RParen) {
		rp := p.advance()
		p.sink.SetEnd(node, rp.End.Offset)
		p.sink.SetTrailing(node, rp.TrailingTrivia)
	} else {
		p.sink.SetHasError(node, true)
		p.emitMissingToken(token.RParen, "parenthesized expression")
	}
	return node
}

func (p *parser[N, S]) parseArrayLiteral() N {
	lb := p.advance() // '{'
	return p.parseBracketedList(KindArrayLiteral, lb, token.RBrace, (*parser[N, S]).parseAssignment)
}
