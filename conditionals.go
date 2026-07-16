package parser

import (
	"strings"

	"github.com/pawnkit/pawn-parser/token"
)

func (p *parser[N, S]) parseConditionalRegion(g itemGrammar[N, S]) N {
	startPos := p.pos
	savedBroken := p.broken
	storageMark := p.sink.Mark()
	region, ok := p.tryParseConditionalRegion(g)
	if ok {
		return region
	}
	p.pos = startPos
	p.broken = savedBroken
	p.sink.Rewind(storageMark)
	if p.isConditionalSplice() {
		return p.consumeConditionalSplice()
	}
	return p.rawConditionalRegion()
}

func (p *parser[N, S]) isConditionalSplice() bool {
	end, braceDelta, ok := p.conditionalRegionExtent(p.pos)
	if !ok {
		return false
	}
	if braceDelta < 0 {
		return true
	}
	return braceDelta > 0 && p.hasClosingConditionalSplice(end)
}

func (p *parser[N, S]) hasClosingConditionalSplice(pos int) bool {
	depth := 0
	for pos < p.toks.len() && p.toks.kind(pos) != token.EOF {
		switch p.toks.kind(pos) {
		case token.LBrace:
			depth++
		case token.RBrace:
			if depth == 0 {
				return false
			}
			depth--
		case token.Hash:
			if classifyDirectiveName(p.peekAt(pos+1).Text(p.source)) == dirIf {
				end, braceDelta, ok := p.conditionalRegionExtent(pos)
				if !ok {
					return false
				}
				if depth == 0 && braceDelta < 0 {
					return true
				}
				pos = end
				continue
			}
			pos = p.afterLogicalLine(pos)
			continue
		default:
			// Other tokens do not affect the surrounding brace depth.
		}
		pos++
	}
	return false
}

func (p *parser[N, S]) conditionalRegionExtent(start int) (end, braceDelta int, ok bool) {
	depth := 0
	for pos := start; pos < p.toks.len(); pos++ {
		tok := p.toks.at(pos)
		if tok.Kind == token.Hash {
			switch classifyDirectiveName(p.peekAt(pos + 1).Text(p.source)) {
			case dirIf:
				depth++
			case dirEndif:
				depth--
				if depth == 0 {
					return p.afterLogicalLine(pos), braceDelta, true
				}
			}
			continue
		}
		if depth > 0 {
			switch tok.Kind {
			case token.LBrace:
				braceDelta++
			case token.RBrace:
				braceDelta--
			default:
				// Other tokens do not contribute to a brace splice.
			}
		}
	}
	return start, 0, false
}

func (p *parser[N, S]) afterLogicalLine(pos int) int {
	for pos < p.toks.len() {
		pos++
		if pos == p.toks.len() || lastTokenEndsLine(p.toks.at(pos-1)) {
			break
		}
	}
	return pos
}

func (p *parser[N, S]) peekAt(pos int) token.Token {
	if pos >= p.toks.len() {
		return p.toks.at(p.toks.len() - 1)
	}
	return p.toks.at(pos)
}

func (p *parser[N, S]) consumeConditionalSplice() N {
	start := p.cur().Start.Offset
	leading := p.cur().LeadingTrivia
	depth := 0
	dummyBracketDepth := 0
	var last token.Token
	for !p.atEnd() {
		if p.at(token.Hash) {
			switch p.peekDirectiveKeyword() {
			case dirIf:
				depth++
			case dirEndif:
				depth--
			}
		}
		last = p.consumeLogicalLineCounting(false, &dummyBracketDepth)
		if depth == 0 {
			break
		}
	}
	return p.sink.Store(Node{
		Kind:     KindConditionalSplice,
		Start:    start,
		End:      last.End.Offset,
		Leading:  leading,
		Trailing: last.TrailingTrivia,
		Raw:      p.source[start:last.End.Offset],
	})
}

func (p *parser[N, S]) tryParseConditionalRegion(g itemGrammar[N, S]) (node N, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, isAbort := r.(condAbort); isAbort {
				node, ok = p.sink.Nil(), false
				return
			}
			panic(r)
		}
	}()

	p.condDepth++
	defer func() { p.condDepth-- }()
	if p.condDepth > maxParseDepth {
		return p.sink.Nil(), false
	}

	region := p.sink.Store(Node{Kind: KindConditionalRegion, Start: p.cur().Start.Offset, Leading: p.cur().LeadingTrivia})
	for {
		if !p.at(token.Hash) {
			return p.sink.Nil(), false
		}
		dk := p.peekDirectiveKeyword()
		directive := p.consumeRawDirectiveLine(p.cur().Start.Offset, directiveNodeKind(dk))
		branch := p.sink.Store(Node{Kind: KindConditionalBranch, Start: p.sink.Start(directive), End: p.sink.End(directive), Leading: p.sink.Leading(directive), Trailing: p.sink.Trailing(directive)})
		p.sink.SetField(branch, fieldDirective, directive)
		p.sink.AddChild(branch, directive)
		p.sink.AddChild(region, branch)

		if dk == dirEndif {
			break
		}
		if dk != dirIf && dk != dirElseif && dk != dirElse {
			return p.sink.Nil(), false
		}

		p.branchTop = true
		items := p.parseItemSequence(g)
		for _, it := range items {
			p.sink.AddChild(branch, it)
		}
	}
	if p.conditionalNeedsSharedFallback(region, p.source) && !p.conditionalFunctionHeaders(region) {
		return p.sink.Nil(), false
	}
	return region, true
}

func (p *parser[N, S]) conditionalNeedsSharedFallback(region N, source []byte) bool {
	for _, branch := range p.sink.Children(region) {
		directive := p.sink.Field(branch, fieldDirective)
		for _, item := range p.sink.Children(branch) {
			if item == directive || p.sink.Kind(item) == KindConditionalRegion ||
				p.sink.Kind(item) == KindSharedConditional || p.sink.Kind(item) == KindConditionalFunction {
				continue
			}
			if p.sink.HasError(item) {
				return true
			}
			if p.sink.Kind(item) == KindIfStatement {
				consequence := p.sink.Field(item, fieldConsequence)
				if consequence != p.sink.Nil() && p.sink.Kind(consequence) == KindEmptyStatement && p.sink.Start(consequence) == p.sink.End(consequence) {
					return true
				}
			}
			switch p.sink.Kind(item) {
			case KindFunctionDeclaration, KindStateStatement, KindIfStatement:
				if p.sink.HasError(item) {
					return true
				}
			case KindExpressionStatement:
				start, end := clampRange(source, p.sink.Start(item), p.sink.End(item))
				if p.sink.HasError(item) && strings.HasPrefix(strings.TrimSpace(string(source[start:end])), "else") {
					return true
				}
			default:
				// Other kinds never need the shared-conditional fallback.
			}
		}
	}
	return false
}

func (p *parser[N, S]) rawConditionalRegion() N {
	startOffset := p.cur().Start.Offset
	leading := p.cur().LeadingTrivia

	var live []bool
	falseCount := 0
	bracketDepth := 0
	pastEndif := false
	var last token.Token

	allLive := func() bool {
		return falseCount == 0
	}

	for !p.atEnd() {
		if p.at(token.Hash) {
			switch p.peekDirectiveKeyword() {
			case dirIf:
				live = append(live, true)
				pastEndif = false
			case dirElseif, dirElse:
				if len(live) > 0 {
					if live[len(live)-1] {
						falseCount++
					}
					live[len(live)-1] = false
				}
			case dirEndif:
				if len(live) > 0 {
					if !live[len(live)-1] {
						falseCount--
					}
					live = live[:len(live)-1]
				}
				if len(live) == 0 {
					pastEndif = true
				}
			}
		}
		last = p.consumeLogicalLineCounting(allLive(), &bracketDepth)
		if pastEndif {
			mark := p.sink.Mark()
			if node := p.finishSharedConditional(startOffset, leading, last, bracketDepth); node != p.sink.Nil() {
				return node
			}
			p.sink.Rewind(mark)
			if bracketDepth <= 0 {
				break
			}
		}
	}
	return p.sink.Store(Node{
		Kind:     KindSharedConditional,
		Start:    startOffset,
		End:      last.End.Offset,
		Leading:  leading,
		Trailing: last.TrailingTrivia,
		Raw:      p.source[startOffset:last.End.Offset],
	})
}

func (p *parser[N, S]) finishSharedConditional(start int, leading []token.Trivia, last token.Token, depth int) N {
	prefix := p.sink.Store(Node{Kind: KindSharedConditionalPrefix, Start: start, End: last.End.Offset, Leading: leading, Trailing: last.TrailingTrivia, Raw: p.source[start:last.End.Offset]})
	if depth == 0 && p.at(token.LBrace) {
		return p.newSharedConditional(prefix, p.parseBlock())
	}
	if depth != 1 || p.atEnd() {
		return p.sink.Nil()
	}
	body := p.sink.Store(Node{Kind: KindBlock, Start: last.End.Offset})
	items := p.parseItemSequence(itemGrammar[N, S]{
		parseMode: itemParseStatement,
		stopKind:  token.RBrace,
	})
	for _, item := range items {
		p.sink.AddChild(body, item)
	}
	if !p.at(token.RBrace) {
		return p.sink.Nil()
	}
	rb := p.advance()
	p.sink.SetEnd(body, rb.End.Offset)
	p.sink.SetTrailing(body, rb.TrailingTrivia)
	return p.newSharedConditional(prefix, body)
}

func (p *parser[N, S]) newSharedConditional(prefix, body N) N {
	node := p.sink.NewNode(KindSharedConditional, prefix, body)
	p.sink.SetField(node, fieldPrefix, prefix)
	p.sink.SetField(node, fieldBody, body)
	p.parseSharedConditionalAlternative(node)
	return node
}

func (p *parser[N, S]) parseSharedConditionalAlternative(node N) {
	if !p.at(token.KwElse) {
		return
	}
	p.advance()
	alternative := p.parseControlledStatement()
	p.sink.SetField(node, fieldAlternative, alternative)
	p.sink.AddChild(node, alternative)
}

func (p *parser[N, S]) consumeLogicalLineCounting(count bool, depth *int) token.Token {
	var last token.Token
	for !p.atEnd() {
		k := p.curKind()
		last = p.advance()
		if count {
			switch k {
			case token.LBrace, token.LParen, token.LBracket:
				*depth++
			case token.RBrace, token.RParen, token.RBracket:
				*depth--
			default:
				// Other tokens don't affect bracket depth.
			}
		}
		if lastTokenEndsLine(last) {
			break
		}
	}
	return last
}
