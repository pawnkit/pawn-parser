package parser

import (
	"slices"

	"github.com/pawnkit/pawn-parser/token"
)

var qualifierKinds = []token.Kind{token.KwPublic, token.KwStock, token.KwStatic, token.KwNative, token.KwForward, token.KwConst, token.KwNew}

func isKeywordToken(k token.Kind) bool {
	return k >= token.KwPublic && k <= token.KwNull
}

func (p *parser) parseDeclaration() *Node {
	if p.at(token.KwEnum) {
		return p.parseEnumDeclaration(nil)
	}

	quals := p.collectQualifiers()
	if p.at(token.KwEnum) {
		return p.parseEnumDeclaration(quals)
	}

	if p.peekIsFunctionDecl() {
		return p.parseFunctionLike(quals)
	}
	if !p.canStartDeclarator() {
		start := p.cur().Start.Offset
		if len(quals) > 0 {
			start = quals[0].Start
		}
		n := &Node{Kind: KindRaw, Start: start, End: p.cur().Start.Offset, HasError: true}
		for _, q := range quals {
			n.addChild(q)
		}
		return n
	}
	return p.parseVariableDeclarationWithQualifiers(quals)
}

func (p *parser) canStartDeclarator() bool {
	saved := p.pos
	defer func() { p.pos = saved }()
	p.parseOptionalTagPrefix()
	return p.at(token.Identifier)
}

func (p *parser) collectQualifiers() []*Node {
	var quals []*Node
	for slices.Contains(qualifierKinds, p.cur().Kind) {
		quals = append(quals, p.newLeaf(KindIdentifier, p.advance()))
	}

	if p.at(token.Identifier) && p.peek(1).Kind == token.Identifier && p.peek(2).Kind == token.LParen {
		quals = append(quals, p.newLeaf(KindIdentifier, p.advance()))
	}
	return quals
}

func (p *parser) peekIsFunctionDecl() bool {
	saved := p.pos
	defer func() { p.pos = saved }()

	p.parseOptionalTagPrefix()
	if p.at(token.KwOperator) {
		p.advance()
		if isOverloadableOperator(p.cur().Kind) {
			p.advance()
		}
		return p.at(token.LParen)
	}
	if !p.at(token.Identifier) {
		return false
	}
	p.advance()
	return p.at(token.LParen)
}

func isOverloadableOperator(k token.Kind) bool {
	switch k {
	case token.Plus, token.Minus, token.Star, token.Slash, token.Percent,
		token.Assign, token.Eq, token.NotEq, token.Lt, token.Gt, token.LtEq, token.GtEq,
		token.Bang, token.PlusPlus, token.MinusMinus:
		return true
	default:
		return false
	}
}

func (p *parser) parseFunctionName() *Node {
	if p.at(token.KwOperator) {
		opKw := p.advance()
		if isOverloadableOperator(p.cur().Kind) {
			symTok := p.advance()
			return &Node{
				Kind: KindIdentifier, Start: opKw.Start.Offset, End: symTok.End.Offset,
				Leading: opKw.LeadingTrivia, Trailing: symTok.TrailingTrivia,
			}
		}
		name := p.newLeaf(KindIdentifier, opKw)
		name.HasError = true
		return name
	}
	return p.newLeaf(KindIdentifier, p.advance())
}
