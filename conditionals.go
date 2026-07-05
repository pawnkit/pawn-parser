package parser

import (
	"strings"

	"github.com/pawnkit/pawn-parser/token"
)

func (p *parser) parseConditionalRegion(g itemGrammar) *Node {
	startPos := p.pos
	savedBroken := p.broken
	region, ok := p.tryParseConditionalRegion(g)
	if ok {
		return region
	}
	p.pos = startPos
	p.broken = savedBroken
	if p.isConditionalSplice() {
		return p.consumeConditionalSplice()
	}
	return p.rawConditionalRegion()
}

func (p *parser) isConditionalSplice() bool {
	end, braceDelta, ok := p.conditionalRegionExtent(p.pos)
	if !ok {
		return false
	}
	if braceDelta < 0 {
		return true
	}
	return braceDelta > 0 && p.hasClosingConditionalSplice(end)
}

func (p *parser) hasClosingConditionalSplice(pos int) bool {
	depth := 0
	for pos < len(p.toks) && p.toks[pos].Kind != token.EOF {
		switch p.toks[pos].Kind {
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

func (p *parser) conditionalRegionExtent(start int) (end, braceDelta int, ok bool) {
	depth := 0
	for pos := start; pos < len(p.toks); pos++ {
		tok := p.toks[pos]
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

func (p *parser) afterLogicalLine(pos int) int {
	for pos < len(p.toks) {
		pos++
		if pos == len(p.toks) || lastTokenEndsLine(p.toks[pos-1]) {
			break
		}
	}
	return pos
}

func (p *parser) peekAt(pos int) token.Token {
	if pos >= len(p.toks) {
		return p.toks[len(p.toks)-1]
	}
	return p.toks[pos]
}

func (p *parser) consumeConditionalSplice() *Node {
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
	return &Node{
		Kind:     KindConditionalSplice,
		Start:    start,
		End:      last.End.Offset,
		Leading:  leading,
		Trailing: last.TrailingTrivia,
		Raw:      p.source[start:last.End.Offset],
	}
}

func (p *parser) tryParseConditionalRegion(g itemGrammar) (node *Node, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, isAbort := r.(condAbort); isAbort {
				node, ok = nil, false
				return
			}
			panic(r)
		}
	}()

	p.condDepth++
	defer func() { p.condDepth-- }()
	if p.condDepth > maxParseDepth {
		return nil, false
	}

	region := &Node{Kind: KindConditionalRegion, Start: p.cur().Start.Offset, Leading: p.cur().LeadingTrivia}
	for {
		if !p.at(token.Hash) {
			return nil, false
		}
		dk := p.peekDirectiveKeyword()
		directive := p.consumeRawDirectiveLine(p.cur().Start.Offset, directiveNodeKind(dk))
		branch := &Node{Kind: KindConditionalBranch, Start: directive.Start, End: directive.End, Leading: directive.Leading, Trailing: directive.Trailing}
		setField(branch, "directive", directive)
		branch.Children = append(branch.Children, directive)
		region.addChild(branch)

		if dk == dirEndif {
			break
		}
		if dk != dirIf && dk != dirElseif && dk != dirElse {
			return nil, false
		}

		p.branchTop = true
		items := p.parseItemSequence(g)
		for _, it := range items {
			branch.addChild(it)
		}
	}
	if conditionalNeedsSharedFallback(region, p.source) && !conditionalFunctionHeaders(region) {
		return nil, false
	}
	return region, true
}

func conditionalNeedsSharedFallback(region *Node, source []byte) bool {
	for _, branch := range region.Children {
		directive := branch.Field("directive")
		for _, item := range branch.Children {
			if item == directive || item.Kind == KindConditionalRegion ||
				item.Kind == KindSharedConditional || item.Kind == KindConditionalFunction {
				continue
			}
			if item.HasError {
				return true
			}
			if item.Kind == KindIfStatement {
				consequence := item.Field("consequence")
				if consequence != nil && consequence.Kind == KindEmptyStatement && consequence.Start == consequence.End {
					return true
				}
			}
			switch item.Kind {
			case KindFunctionDeclaration, KindStateStatement, KindIfStatement:
				if item.HasError {
					return true
				}
			case KindExpressionStatement:
				if item.HasError && strings.HasPrefix(strings.TrimSpace(item.Text(source)), "else") {
					return true
				}
			default:
				// Other kinds never need the shared-conditional fallback.
			}
		}
	}
	return false
}

func (p *parser) rawConditionalRegion() *Node {
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
			if node := p.finishSharedConditional(startOffset, leading, last, bracketDepth); node != nil {
				return node
			}
			if bracketDepth <= 0 {
				break
			}
		}
	}
	return &Node{
		Kind:     KindSharedConditional,
		Start:    startOffset,
		End:      last.End.Offset,
		Leading:  leading,
		Trailing: last.TrailingTrivia,
		Raw:      p.source[startOffset:last.End.Offset],
	}
}

func (p *parser) finishSharedConditional(start int, leading []token.Trivia, last token.Token, depth int) *Node {
	prefix := &Node{Kind: KindSharedConditionalPrefix, Start: start, End: last.End.Offset, Leading: leading, Trailing: last.TrailingTrivia, Raw: p.source[start:last.End.Offset]}
	if depth == 0 && p.at(token.LBrace) {
		return p.newSharedConditional(prefix, p.parseBlock())
	}
	if depth != 1 || p.atEnd() {
		return nil
	}
	body := &Node{Kind: KindBlock, Start: last.End.Offset}
	items := p.parseItemSequence(itemGrammar{
		parseItem: func(p *parser) *Node { return p.parseStatement() },
		stop:      func(p *parser) bool { return p.at(token.RBrace) },
	})
	for _, item := range items {
		body.addChild(item)
	}
	if !p.at(token.RBrace) {
		return nil
	}
	rb := p.advance()
	body.End = rb.End.Offset
	body.Trailing = rb.TrailingTrivia
	return p.newSharedConditional(prefix, body)
}

func (p *parser) newSharedConditional(prefix, body *Node) *Node {
	node := p.newNode(KindSharedConditional, prefix, body)
	setField(node, "prefix", prefix)
	setField(node, "body", body)
	p.parseSharedConditionalAlternative(node)
	return node
}

func (p *parser) parseSharedConditionalAlternative(node *Node) {
	if !p.at(token.KwElse) {
		return
	}
	p.advance()
	alternative := p.parseControlledStatement()
	setField(node, "alternative", alternative)
	node.addChild(alternative)
}

func (p *parser) consumeLogicalLineCounting(count bool, depth *int) token.Token {
	var last token.Token
	for !p.atEnd() {
		k := p.cur().Kind
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
