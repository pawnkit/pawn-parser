package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser[N, S]) parseSingleDirective() N {
	startOffset := p.cur().Start.Offset
	dk := p.peekDirectiveKeyword()
	switch dk {
	case dirDefine:
		return p.parseDefineDirective(startOffset)
	case dirInclude:
		return p.parseIncludeDirective(startOffset, KindDirectiveInclude)
	case dirTryInclude:
		return p.parseIncludeDirective(startOffset, KindDirectiveTryInclude)
	default:
		return p.consumeRawDirectiveLine(startOffset, directiveNodeKind(dk))
	}
}

func (p *parser[N, S]) consumeRawDirectiveLine(startOffset int, kind Kind) N {
	leading := p.cur().LeadingTrivia
	last := p.advance() // '#'
	if !p.atEnd() {
		last = p.advance() // directive keyword
	}
	payloadStartIdx := p.pos
	if !lastTokenEndsLine(last) {
		for !p.atEnd() {
			last = p.advance()
			if lastTokenEndsLine(last) {
				break
			}
		}
	}
	payloadEndIdx := p.pos
	end := max(last.End.Offset, startOffset)
	n := p.directiveSpan(kind, startOffset, end, leading, last.TrailingTrivia)

	if kind == KindDirectiveIf || kind == KindDirectiveElseif || kind == KindDirectiveAssert {
		if cond, ok := p.trySubParseExpression(payloadStartIdx, payloadEndIdx); ok {
			p.sink.SetField(n, fieldCondition, cond)
			p.sink.SetChildren(n, []N{cond})
		}
	}
	return n
}

func (p *parser[N, S]) trySubParseExpression(startIdx, endIdx int) (N, bool) {
	if startIdx >= endIdx {
		return p.sink.Nil(), false
	}
	toks := make([]token.Token, endIdx-startIdx, endIdx-startIdx+1)
	copy(toks, p.toks[startIdx:endIdx])
	last := toks[len(toks)-1]
	toks = append(toks, token.Token{Kind: token.EOF, Start: last.End, End: last.End})
	return p.tryParseAll(toks, false, (*parser[N, S]).parseExpression)
}

func (p *parser[N, S]) parseIncludeDirective(startOffset int, kind Kind) N {
	leading := p.cur().LeadingTrivia
	p.advance()
	p.advance()
	if p.atEnd() || lastTokenEndsLine(p.toks[p.pos-1]) {
		return p.directiveSpan(kind, startOffset, p.toks[p.pos-1].End.Offset, leading, p.toks[p.pos-1].TrailingTrivia)
	}

	pathStart := p.cur().Start.Offset
	var last token.Token
	for !p.atEnd() {
		last = p.advance()
		if lastTokenEndsLine(last) {
			break
		}
	}
	pathEnd := last.End.Offset
	pathNode := p.directiveSpan(KindDirectivePath, pathStart, pathEnd, nil, last.TrailingTrivia)
	p.sink.SetTrailing(pathNode, last.TrailingTrivia)

	node := p.sink.Store(Node{Kind: kind, Start: startOffset, End: pathEnd, Leading: leading, Trailing: last.TrailingTrivia})
	p.sink.SetChildren(node, []N{pathNode})
	p.sink.SetField(node, fieldPath, pathNode)
	return node
}

type condAbort struct{}
