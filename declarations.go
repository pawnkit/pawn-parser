package parser

import (
	"slices"

	"github.com/pawnkit/pawn-parser/token"
)

var qualifierKinds = []token.Kind{token.KwPublic, token.KwStock, token.KwStatic, token.KwNative, token.KwForward, token.KwConst, token.KwNew}

func isKeywordToken(k token.Kind) bool {
	return k >= token.KwPublic && k <= token.KwNull
}

func (p *parser[N, S]) parseDeclaration() N {
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
			start = p.sink.Start(quals[0])
		}
		n := p.sink.Store(Node{Kind: KindRaw, Start: start, End: p.cur().Start.Offset, HasError: true})
		for _, q := range quals {
			p.sink.AddChild(n, q)
		}
		return n
	}
	return p.parseVariableDeclarationWithQualifiers(quals)
}

func (p *parser[N, S]) peekIsOperatorMacroInvocation() bool {
	if !p.at(token.Identifier) || p.peekKind(1) != token.LParen {
		return false
	}
	switch p.peekKind(2) {
	case token.Plus, token.Minus, token.Star, token.Slash, token.Percent,
		token.PlusPlus, token.MinusMinus, token.Eq, token.NotEq,
		token.Lt, token.Gt, token.LtEq, token.GtEq, token.Bang, token.Tilde:
		return true
	default:
		return false
	}
}

func (p *parser[N, S]) parseOperatorMacroInvocation() N {
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
				return p.directiveSpan(KindMacroInvocation, start, last.End.Offset, leading, last.TrailingTrivia)
			}
		default:
		}
	}
	return p.directiveSpan(KindMacroInvocation, start, last.End.Offset, leading, last.TrailingTrivia)
}

func (p *parser[N, S]) canStartDeclarator() bool {
	saved := p.pos
	mark := p.sink.Mark()
	defer func() {
		p.pos = saved
		p.sink.Rewind(mark)
	}()
	p.parseOptionalTagPrefix()
	return p.at(token.Identifier)
}

func (p *parser[N, S]) collectQualifiers() []N {
	var quals []N
	for {
		switch {
		case p.annotationQualifierStart():
			quals = p.sink.Append(quals, p.parseAnnotationQualifier())
		case slices.Contains(qualifierKinds, p.curKind()):
			quals = p.sink.Append(quals, p.sink.NewLeaf(KindIdentifier, p.advance()))
		case p.macroFunctionQualifierStart():
			quals = p.sink.Append(quals, p.sink.NewLeaf(KindIdentifier, p.advance()))
		default:
			return quals
		}
	}
}

func (p *parser[N, S]) parseAnnotationQualifier() N {
	name := p.sink.NewLeaf(KindIdentifier, p.advance())
	return p.parseCall(name)
}

func (p *parser[N, S]) annotationQualifierStart() bool {
	if !p.at(token.Identifier) || p.peekKind(1) != token.LParen || p.cur().Text(p.source)[0] != '@' {
		return false
	}
	saved := p.pos
	mark := p.sink.Mark()
	defer func() {
		p.pos = saved
		p.sink.Rewind(mark)
	}()
	p.advance()
	p.parseArgumentList()
	return p.peekIsFunctionDecl() || p.macroFunctionQualifierStart()
}

func (p *parser[N, S]) macroFunctionQualifierStart() bool {
	if !p.at(token.Identifier) {
		return false
	}
	saved := p.pos
	defer func() { p.pos = saved }()
	p.advance()
	for p.at(token.ColonColon) && p.peekKind(1) == token.Identifier {
		p.advance()
		p.advance()
	}
	for slices.Contains(qualifierKinds, p.curKind()) {
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

func (p *parser[N, S]) peekIsFunctionDecl() bool {
	saved := p.pos
	mark := p.sink.Mark()
	defer func() {
		p.pos = saved
		p.sink.Rewind(mark)
	}()

	p.parseOptionalTagPrefix()
	p.parseDimensions()
	if p.at(token.KwOperator) {
		p.advance()
		if isOverloadableOperator(p.curKind()) {
			p.advance()
		}
		return p.at(token.LParen)
	}
	if !isFunctionNameToken(p.curKind()) {
		return false
	}
	p.advance()
	for p.at(token.ColonColon) && p.peekKind(1) == token.Identifier {
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

func (p *parser[N, S]) parseFunctionName() N {
	if p.at(token.KwOperator) {
		opKw := p.advance()
		if isOverloadableOperator(p.curKind()) {
			symTok := p.advance()
			return p.sink.Store(Node{
				Kind: KindIdentifier, Start: opKw.Start.Offset, End: symTok.End.Offset,
				Leading: opKw.LeadingTrivia, Trailing: symTok.TrailingTrivia,
			})
		}
		name := p.sink.NewLeaf(KindIdentifier, opKw)
		p.sink.SetHasError(name, true)
		return name
	}
	validName := isFunctionNameToken(p.curKind())
	name := p.parseQualifiedIdentifier()
	if !validName {
		p.sink.SetHasError(name, true)
	}
	return name
}
