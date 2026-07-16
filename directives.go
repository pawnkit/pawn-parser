package parser

import "github.com/pawnkit/pawn-parser/token"

type itemGrammar struct {
	parseItem                 func(p *parser) *Node
	stop                      func(p *parser) bool
	stopKind                  token.Kind
	abortAtStop               bool
	commaSeparated            bool
	preserveRecoverySemicolon bool
	parseUnknownHashAsItem    bool
	recoveryContext           string
	recoveryExpected          []token.Kind
}

func lastTokenEndsLine(t token.Token) bool {
	for _, tr := range t.TrailingTrivia {
		if tr.Kind == token.Newline {
			return true
		}
	}
	return false
}

type directiveKeyword int

const (
	dirUnknown directiveKeyword = iota
	dirInclude
	dirTryInclude
	dirDefine
	dirUndef
	dirIf
	dirElseif
	dirElse
	dirEndif
	dirPragma
	dirError
	dirWarning
	dirEmit
	dirAssert
	dirLine
	dirFile
	dirEndinput
)

func classifyDirectiveName(name string) directiveKeyword {
	switch name {
	case "include":
		return dirInclude
	case "tryinclude":
		return dirTryInclude
	case "define":
		return dirDefine
	case "undef":
		return dirUndef
	case "if":
		return dirIf
	case "elseif":
		return dirElseif
	case "else":
		return dirElse
	case "endif":
		return dirEndif
	case "pragma":
		return dirPragma
	case "error":
		return dirError
	case "warning":
		return dirWarning
	case "emit":
		return dirEmit
	case "assert":
		return dirAssert
	case "line":
		return dirLine
	case "file":
		return dirFile
	case "endinput":
		return dirEndinput
	default:
		return dirUnknown
	}
}

func directiveNodeKind(dk directiveKeyword) Kind {
	switch dk {
	case dirUndef:
		return KindDirectiveUndef
	case dirIf:
		return KindDirectiveIf
	case dirElseif:
		return KindDirectiveElseif
	case dirElse:
		return KindDirectiveElse
	case dirEndif:
		return KindDirectiveEndif
	case dirPragma:
		return KindDirectivePragma
	case dirError:
		return KindDirectiveError
	case dirWarning:
		return KindDirectiveWarning
	case dirEmit:
		return KindDirectiveEmit
	case dirAssert:
		return KindDirectiveAssert
	case dirLine:
		return KindDirectiveLine
	case dirFile:
		return KindDirectiveFile
	case dirEndinput:
		return KindDirectiveEndinput
	default:
		return KindDirectiveRaw
	}
}

// peekDirectiveKeyword assumes p.cur() is a Hash token and reports the
// directive keyword that follows it, without consuming anything.
func (p *parser) peekDirectiveKeyword() directiveKeyword {
	kw := p.peek(1)
	return classifyDirectiveName(kw.Text(p.source))
}

func (p *parser) parseItemSequence(g itemGrammar) []*Node {
	isBranchTop := p.branchTop
	p.branchTop = false

	var items []*Node
	for !p.atEnd() {
		if p.at(token.Hash) {
			if dk := p.peekDirectiveKeyword(); dk == dirElseif || dk == dirElse || dk == dirEndif {
				if isBranchTop || p.condDepth == 0 {
					return items
				}
				panic(condAbort{})
			}
		}
		if p.itemSequenceStopped(g) {
			return items
		}
		startPos := p.pos
		var item *Node
		if p.at(token.Hash) {
			switch p.peekDirectiveKeyword() {
			case dirIf:
				item = p.parseConditionalRegion(g)
			case dirUnknown:
				if g.parseUnknownHashAsItem {
					item = p.parseGrammarItem(g)
				} else {
					item = p.parseSingleDirective()
				}
			default:
				item = p.parseSingleDirective()
			}
		} else {
			item = p.parseGrammarItem(g)
		}
		if p.pos == startPos {
			if recovered := p.recoverStuckItem(g); recovered != nil {
				items = p.appendNode(items, recovered)
			}
			continue
		}
		if item != nil {
			if p.attachConditionalContinuation(items, item) {
				continue
			}
			p.attachSharedAlternative(item)
			if item.Kind == KindConditionalRegion && p.at(token.LBrace) && conditionalFunctionHeaders(item) {
				body := p.parseBlock()
				wrapper := p.newNode(KindConditionalFunction, item, body)
				p.setField(wrapper, "headers", item)
				p.setField(wrapper, "body", body)
				item = wrapper
			}
			items = p.appendNode(items, item)
		}
	}
	return items
}

func (p *parser) itemSequenceStopped(g itemGrammar) bool {
	if g.abortAtStop {
		p.abortIfSharedAcrossBranch()
	}
	if g.stopKind != token.Invalid && p.at(g.stopKind) {
		return true
	}
	return g.stop != nil && g.stop(p)
}

func (p *parser) parseGrammarItem(g itemGrammar) *Node {
	item := g.parseItem(p)
	if g.commaSeparated && p.at(token.Comma) {
		comma := p.advance()
		p.mergeCommaTrivia(item, comma)
	}
	return item
}

func (p *parser) attachSharedAlternative(conditional *Node) {
	if conditional.Kind != KindConditionalRegion || !p.at(token.KwElse) {
		return
	}
	p.advance()
	alternative := p.parseControlledStatement()
	p.setField(conditional, "alternative", alternative)
	for _, branch := range conditional.Children {
		ifStatement := trailingBranchIf(branch)
		if ifStatement == nil || ifStatement.Field("alternative") != nil {
			continue
		}
		p.setField(ifStatement, "alternative", alternative)
		p.setField(branch, "shared_alternative", alternative)
	}
	conditional.End = alternative.End
	conditional.Trailing = alternative.Trailing
}

func trailingBranchIf(branch *Node) *Node {
	for i := len(branch.Children) - 1; i >= 0; i-- {
		child := branch.Children[i]
		if child.Kind.IsDirective() {
			continue
		}
		if child.Kind == KindIfStatement {
			return child
		}
		return nil
	}
	return nil
}

func (p *parser) attachConditionalContinuation(items []*Node, conditional *Node) bool {
	if len(items) == 0 || conditional.Kind != KindSharedConditional || !p.at(token.KwElse) {
		return false
	}
	previous := items[len(items)-1]
	if previous.Kind != KindIfStatement || previous.Field("alternative") != nil {
		return false
	}
	p.setField(previous, "conditional_alternatives", conditional)
	p.addChild(previous, conditional)
	p.advance()
	alternative := p.parseControlledStatement()
	p.setField(previous, "alternative", alternative)
	p.addChild(previous, alternative)
	return true
}

func conditionalFunctionHeaders(region *Node) bool {
	found := false
	var visit func(*Node) bool
	visit = func(n *Node) bool {
		switch n.Kind {
		case KindConditionalRegion, KindConditionalBranch:
			for _, child := range n.Children {
				if !visit(child) {
					return false
				}
			}
			n.HasError = false
			return true
		case KindDirectiveIf, KindDirectiveElseif, KindDirectiveElse, KindDirectiveEndif:
			return true
		case KindFunctionDeclaration:
			if n.Field("alias") != nil {
				return false
			}
			n.HasError = false
			n.MissingSemi = true
			found = true
			return true
		default:
			return false
		}
	}
	return visit(region) && found
}

func (p *parser) parseBracketedList(kind Kind, open token.Token, closeTok token.Kind, parseItem func(*parser) *Node) *Node {
	node := p.storeNode(Node{Kind: kind, Start: open.Start.Offset, Leading: open.LeadingTrivia})
	items := p.parseItemSequence(itemGrammar{
		parseItem:              parseItem,
		commaSeparated:         true,
		parseUnknownHashAsItem: true,
		recoveryContext:        "list item",
		recoveryExpected:       []token.Kind{token.Comma, closeTok},
		stopKind:               closeTok,
		abortAtStop:            true,
	})
	for _, it := range items {
		p.addChild(node, it)
	}
	if p.at(closeTok) {
		closeToken := p.advance()
		node.End = closeToken.End.Offset
		node.Trailing = closeToken.TrailingTrivia
	} else {
		node.HasError = true
		p.emitMissingToken(closeTok, kind.String())
	}
	return node
}

func (p *parser) mergeCommaTrivia(item *Node, comma token.Token) {
	if item == nil {
		return
	}
	if len(comma.LeadingTrivia) == 0 && len(comma.TrailingTrivia) == 0 {
		return
	}
	merged := p.storage.trivia.alloc(len(item.Trailing) + len(comma.LeadingTrivia) + len(comma.TrailingTrivia))
	offset := copy(merged, item.Trailing)
	offset += copy(merged[offset:], comma.LeadingTrivia)
	copy(merged[offset:], comma.TrailingTrivia)
	item.Trailing = merged
}
