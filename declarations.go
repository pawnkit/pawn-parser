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
	if p.peekIsOperatorMacroInvocation() {
		return p.parseOperatorMacroInvocation()
	}
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
	if !p.canStartDeclarator() && (len(quals) == 0 || !p.at(token.Hash) || p.peekDirectiveKeyword() != dirIf) {
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

func (p *parser) peekIsOperatorMacroInvocation() bool {
	if !p.at(token.Identifier) || p.peek(1).Kind != token.LParen {
		return false
	}
	switch p.peek(2).Kind {
	case token.Plus, token.Minus, token.Star, token.Slash, token.Percent,
		token.PlusPlus, token.MinusMinus, token.Eq, token.NotEq,
		token.Lt, token.Gt, token.LtEq, token.GtEq, token.Bang, token.Tilde:
		return true
	default:
		return false
	}
}

func (p *parser) parseOperatorMacroInvocation() *Node {
	start := p.cur().Start.Offset
	leading := p.cur().LeadingTrivia
	depth := 0
	last := p.cur()
	for !p.atEnd() {
		last = p.advance()
		switch last.Kind {
		case token.LParen:
			depth++
		case token.RParen:
			depth--
		case token.Semicolon:
			if depth == 0 {
				return directiveSpan(p.source, KindMacroInvocation, start, last.End.Offset, leading, last.TrailingTrivia)
			}
		default:
		}
	}
	return directiveSpan(p.source, KindMacroInvocation, start, last.End.Offset, leading, last.TrailingTrivia)
}

func (p *parser) canStartDeclarator() bool {
	saved := p.pos
	defer func() { p.pos = saved }()
	p.parseOptionalTagPrefix()
	return p.at(token.Identifier)
}

func (p *parser) collectQualifiers() []*Node {
	var quals []*Node
	for {
		switch {
		case p.annotationQualifierStart():
			quals = append(quals, p.parseAnnotationQualifier())
		case slices.Contains(qualifierKinds, p.cur().Kind):
			quals = append(quals, p.newLeaf(KindIdentifier, p.advance()))
		case p.macroFunctionQualifierStart():
			quals = append(quals, p.newLeaf(KindIdentifier, p.advance()))
		default:
			return quals
		}
	}
}

func (p *parser) parseAnnotationQualifier() *Node {
	name := p.newLeaf(KindIdentifier, p.advance())
	return p.parseCall(name)
}

func (p *parser) annotationQualifierStart() bool {
	if !p.at(token.Identifier) || p.peek(1).Kind != token.LParen || p.cur().Text(p.source)[0] != '@' {
		return false
	}
	saved := p.pos
	defer func() { p.pos = saved }()
	p.advance()
	p.parseArgumentList()
	return p.peekIsFunctionDecl() || p.macroFunctionQualifierStart()
}

func (p *parser) macroFunctionQualifierStart() bool {
	if !p.at(token.Identifier) {
		return false
	}
	saved := p.pos
	defer func() { p.pos = saved }()
	p.advance()
	for p.at(token.ColonColon) && p.peek(1).Kind == token.Identifier {
		p.advance()
		p.advance()
	}
	for slices.Contains(qualifierKinds, p.cur().Kind) {
		p.advance()
	}
	for {
		if p.peekIsFunctionDecl() {
			return true
		}
		if !p.at(token.Identifier) {
			return false
		}
		p.advance()
	}
}

func isFunctionNameToken(kind token.Kind) bool {
	return kind == token.Identifier || isKeywordToken(kind)
}

func (p *parser) peekIsFunctionDecl() bool {
	saved := p.pos
	defer func() { p.pos = saved }()

	p.parseOptionalTagPrefix()
	p.parseDimensions()
	if p.at(token.KwOperator) {
		p.advance()
		if isOverloadableOperator(p.cur().Kind) {
			p.advance()
		}
		return p.at(token.LParen)
	}
	if !isFunctionNameToken(p.cur().Kind) {
		return false
	}
	p.advance()
	for p.at(token.ColonColon) && p.peek(1).Kind == token.Identifier {
		p.advance()
		p.advance()
	}
	p.parseDimensions()
	if p.at(token.Lt) {
		p.parseStateSelector()
	}
	return p.at(token.LParen)
}

func isOverloadableOperator(k token.Kind) bool {
	switch k {
	case token.Plus, token.Minus, token.Star, token.Slash, token.Percent, token.Tilde,
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
	validName := isFunctionNameToken(p.cur().Kind)
	name := p.parseQualifiedIdentifier()
	if !validName {
		name.HasError = true
	}
	return name
}
