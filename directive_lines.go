package parser

import "github.com/pawnkit/pawn-parser/token"

func (p *parser) parseSingleDirective() *Node {
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

func (p *parser) consumeRawDirectiveLine(startOffset int, kind Kind) *Node {
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
	n := directiveSpan(p.source, kind, startOffset, end, leading, last.TrailingTrivia)

	if kind == KindDirectiveIf || kind == KindDirectiveElseif || kind == KindDirectiveAssert {
		if cond, ok := p.trySubParseExpression(payloadStartIdx, payloadEndIdx); ok {
			setField(n, "condition", cond)
			n.Children = []*Node{cond}
		}
	}
	return n
}

func (p *parser) trySubParseExpression(startIdx, endIdx int) (*Node, bool) {
	if startIdx >= endIdx {
		return nil, false
	}
	toks := make([]token.Token, endIdx-startIdx, endIdx-startIdx+1)
	copy(toks, p.toks[startIdx:endIdx])
	last := toks[len(toks)-1]
	toks = append(toks, token.Token{Kind: token.EOF, Start: last.End, End: last.End})
	return tryParseAll(toks, p.source, false, (*parser).parseExpression)
}

func (p *parser) parseIncludeDirective(startOffset int, kind Kind) *Node {
	leading := p.cur().LeadingTrivia
	p.advance()
	p.advance()
	if p.atEnd() || lastTokenEndsLine(p.toks[p.pos-1]) {
		return directiveSpan(p.source, kind, startOffset, p.toks[p.pos-1].End.Offset, leading, p.toks[p.pos-1].TrailingTrivia)
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
	pathNode := rawNode(p.source, pathStart, pathEnd)
	pathNode.HasError = false
	pathNode.Trailing = last.TrailingTrivia

	node := &Node{Kind: kind, Start: startOffset, End: pathEnd, Leading: leading, Trailing: last.TrailingTrivia}
	node.Children = []*Node{pathNode}
	setField(node, "path", pathNode)
	return node
}

type condAbort struct{}
