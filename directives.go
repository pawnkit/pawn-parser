package parser

import "github.com/pawnkit/pawn-parser/token"

type itemGrammar struct {
	parseItem                 func(p *parser) *Node
	stop                      func(p *parser) bool
	preserveRecoverySemicolon bool
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
		if g.stop(p) {
			return items
		}
		startPos := p.pos
		var item *Node
		if p.at(token.Hash) {
			switch p.peekDirectiveKeyword() {
			case dirIf:
				item = p.parseConditionalRegion(g)
			default:
				item = p.parseSingleDirective()
			}
		} else {
			item = g.parseItem(p)
		}
		if p.pos == startPos {
			if recovered := p.recoverStuckItem(g.preserveRecoverySemicolon); recovered != nil {
				items = append(items, recovered)
			}
			continue
		}
		if item != nil {
			if item.Kind == KindConditionalRegion && p.at(token.LBrace) && conditionalFunctionHeaders(item) {
				body := p.parseBlock()
				wrapper := p.newNode(KindConditionalFunction, item, body)
				setField(wrapper, "headers", item)
				setField(wrapper, "body", body)
				item = wrapper
			}
			items = append(items, item)
		}
	}
	return items
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

func parseCommaListItem(parseOne func(*parser) *Node) func(*parser) *Node {
	return func(p *parser) *Node {
		item := parseOne(p)
		if p.at(token.Comma) {
			comma := p.advance()
			mergeCommaTrivia(item, comma)
		}
		return item
	}
}

func (p *parser) parseBracketedList(kind Kind, open token.Token, closeTok token.Kind, parseItem func(*parser) *Node) *Node {
	node := &Node{Kind: kind, Start: open.Start.Offset, Leading: open.LeadingTrivia}
	items := p.parseItemSequence(itemGrammar{
		parseItem: parseCommaListItem(parseItem),
		stop: func(p *parser) bool {
			p.abortIfSharedAcrossBranch()
			return p.at(closeTok)
		},
	})
	for _, it := range items {
		node.addChild(it)
	}
	if p.at(closeTok) {
		closeToken := p.advance()
		node.End = closeToken.End.Offset
		node.Trailing = closeToken.TrailingTrivia
	} else {
		node.HasError = true
	}
	return node
}

func mergeCommaTrivia(item *Node, comma token.Token) {
	if item == nil {
		return
	}
	if len(comma.LeadingTrivia) == 0 && len(comma.TrailingTrivia) == 0 {
		return
	}
	merged := make([]token.Trivia, 0, len(item.Trailing)+len(comma.LeadingTrivia)+len(comma.TrailingTrivia))
	merged = append(merged, item.Trailing...)
	merged = append(merged, comma.LeadingTrivia...)
	merged = append(merged, comma.TrailingTrivia...)
	item.Trailing = merged
}
