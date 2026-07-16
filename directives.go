package parser

import "github.com/pawnkit/pawn-parser/token"

type itemGrammar[N comparable, S nodeSink[N]] struct {
	parseItem                 func(p *parser[N, S]) N
	parseMode                 itemParseMode
	stop                      func(p *parser[N, S]) bool
	stopKind                  token.Kind
	abortAtStop               bool
	commaSeparated            bool
	preserveRecoverySemicolon bool
	parseUnknownHashAsItem    bool
	recoveryContext           string
	recoveryExpected          []token.Kind
}

type itemParseMode uint8

const (
	itemParseCustom itemParseMode = iota
	itemParseDeclaration
	itemParseStatement
	itemParseParameter
	itemParseDeclarator
	itemParseEnumEntry
	itemParseSwitchClause
)

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
func (p *parser[N, S]) peekDirectiveKeyword() directiveKeyword {
	kw := p.peek(1)
	return classifyDirectiveName(kw.Text(p.source))
}

func (p *parser[N, S]) parseItemSequence(g itemGrammar[N, S]) []N {
	isBranchTop := p.branchTop
	p.branchTop = false

	var items []N
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
		var item N
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
			if recovered := p.recoverStuckItem(g); recovered != p.sink.Nil() {
				items = p.sink.Append(items, recovered)
			}
			continue
		}
		if item != p.sink.Nil() {
			if p.attachConditionalContinuation(items, item) {
				continue
			}
			p.attachSharedAlternative(item)
			if p.sink.Kind(item) == KindConditionalRegion && p.at(token.LBrace) && p.conditionalFunctionHeaders(item) {
				body := p.parseBlock()
				wrapper := p.sink.NewNode(KindConditionalFunction, item, body)
				p.sink.SetField(wrapper, fieldHeaders, item)
				p.sink.SetField(wrapper, fieldBody, body)
				item = wrapper
			}
			items = p.sink.Append(items, item)
		}
	}
	return items
}

func (p *parser[N, S]) itemSequenceStopped(g itemGrammar[N, S]) bool {
	if g.abortAtStop {
		p.abortIfSharedAcrossBranch()
	}
	if g.stopKind != token.Invalid && p.at(g.stopKind) {
		return true
	}
	return g.stop != nil && g.stop(p)
}

func (p *parser[N, S]) parseGrammarItem(g itemGrammar[N, S]) N {
	var item N
	switch g.parseMode {
	case itemParseDeclaration:
		item = p.parseDeclaration()
	case itemParseStatement:
		item = p.parseStatement()
	case itemParseParameter:
		item = p.parseParameter()
	case itemParseDeclarator:
		item = p.parseDeclarator()
	case itemParseEnumEntry:
		item = p.parseEnumEntry()
	case itemParseSwitchClause:
		item = p.parseSwitchClause()
	default:
		item = g.parseItem(p)
	}
	if g.commaSeparated && p.at(token.Comma) {
		comma := p.advance()
		p.mergeCommaTrivia(item, comma)
	}
	return item
}

func (p *parser[N, S]) attachSharedAlternative(conditional N) {
	if p.sink.Kind(conditional) != KindConditionalRegion || !p.at(token.KwElse) {
		return
	}
	p.advance()
	alternative := p.parseControlledStatement()
	p.sink.SetField(conditional, fieldAlternative, alternative)
	for _, branch := range p.sink.Children(conditional) {
		ifStatement := p.trailingBranchIf(branch)
		if ifStatement == p.sink.Nil() || p.sink.Field(ifStatement, fieldAlternative) != p.sink.Nil() {
			continue
		}
		p.sink.SetField(ifStatement, fieldAlternative, alternative)
		p.sink.SetField(branch, fieldSharedAlternative, alternative)
	}
	p.sink.SetEnd(conditional, p.sink.End(alternative))
	p.sink.SetTrailing(conditional, p.sink.Trailing(alternative))
}

func (p *parser[N, S]) trailingBranchIf(branch N) N {
	children := p.sink.Children(branch)
	for i := len(children) - 1; i >= 0; i-- {
		child := children[i]
		if p.sink.Kind(child).IsDirective() {
			continue
		}
		if p.sink.Kind(child) == KindIfStatement {
			return child
		}
		return p.sink.Nil()
	}
	return p.sink.Nil()
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

func (p *parser[N, S]) attachConditionalContinuation(items []N, conditional N) bool {
	if len(items) == 0 || p.sink.Kind(conditional) != KindSharedConditional || !p.at(token.KwElse) {
		return false
	}
	previous := items[len(items)-1]
	if p.sink.Kind(previous) != KindIfStatement || p.sink.Field(previous, fieldAlternative) != p.sink.Nil() {
		return false
	}
	p.sink.SetField(previous, fieldConditionalAlternatives, conditional)
	p.sink.AddChild(previous, conditional)
	p.advance()
	alternative := p.parseControlledStatement()
	p.sink.SetField(previous, fieldAlternative, alternative)
	p.sink.AddChild(previous, alternative)
	return true
}

func (p *parser[N, S]) conditionalFunctionHeaders(region N) bool {
	found := false
	var visit func(N) bool
	visit = func(n N) bool {
		switch p.sink.Kind(n) {
		case KindConditionalRegion, KindConditionalBranch:
			for _, child := range p.sink.Children(n) {
				if !visit(child) {
					return false
				}
			}
			p.sink.SetHasError(n, false)
			return true
		case KindDirectiveIf, KindDirectiveElseif, KindDirectiveElse, KindDirectiveEndif:
			return true
		case KindFunctionDeclaration:
			if p.sink.Field(n, fieldAlias) != p.sink.Nil() {
				return false
			}
			p.sink.SetHasError(n, false)
			p.sink.SetMissingSemi(n, true)
			found = true
			return true
		default:
			return false
		}
	}
	return visit(region) && found
}

func (p *parser[N, S]) parseBracketedList(kind Kind, open token.Token, closeTok token.Kind, parseItem func(*parser[N, S]) N) N {
	node := p.sink.Store(Node{Kind: kind, Start: open.Start.Offset, Leading: open.LeadingTrivia})
	items := p.parseItemSequence(itemGrammar[N, S]{
		parseItem:              parseItem,
		commaSeparated:         true,
		parseUnknownHashAsItem: true,
		recoveryContext:        "list item",
		recoveryExpected:       []token.Kind{token.Comma, closeTok},
		stopKind:               closeTok,
		abortAtStop:            true,
	})
	for _, it := range items {
		p.sink.AddChild(node, it)
	}
	if p.at(closeTok) {
		closeToken := p.advance()
		p.sink.SetEnd(node, closeToken.End.Offset)
		p.sink.SetTrailing(node, closeToken.TrailingTrivia)
	} else {
		p.sink.SetHasError(node, true)
		p.emitMissingToken(closeTok, kind.String())
	}
	return node
}

func (p *parser[N, S]) mergeCommaTrivia(item N, comma token.Token) {
	if item == p.sink.Nil() {
		return
	}
	if len(comma.LeadingTrivia) == 0 && len(comma.TrailingTrivia) == 0 {
		return
	}
	merged := p.sink.AllocTrivia(len(p.sink.Trailing(item)) + len(comma.LeadingTrivia) + len(comma.TrailingTrivia))
	offset := copy(merged, p.sink.Trailing(item))
	offset += copy(merged[offset:], comma.LeadingTrivia)
	copy(merged[offset:], comma.TrailingTrivia)
	p.sink.SetTrailing(item, merged)
}
