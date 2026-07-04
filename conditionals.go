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
	return p.rawConditionalRegion()
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
			switch item.Kind {
			case KindFunctionDeclaration, KindStateStatement:
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
			if bracketDepth == 1 && !p.atEnd() {
				prefix := &Node{Kind: KindSharedConditionalPrefix, Start: startOffset, End: last.End.Offset, Leading: leading, Trailing: last.TrailingTrivia, Raw: p.source[startOffset:last.End.Offset]}
				body := &Node{Kind: KindBlock, Start: last.End.Offset}
				items := p.parseItemSequence(itemGrammar{
					parseItem: func(p *parser) *Node { return p.parseStatement() },
					stop:      func(p *parser) bool { return p.at(token.RBrace) },
				})
				for _, item := range items {
					body.addChild(item)
				}
				if p.at(token.RBrace) {
					rb := p.advance()
					body.End = rb.End.Offset
					body.Trailing = rb.TrailingTrivia
					node := p.newNode(KindSharedConditional, prefix, body)
					setField(node, "prefix", prefix)
					setField(node, "body", body)
					return node
				}
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
