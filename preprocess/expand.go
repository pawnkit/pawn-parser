package preprocess

import (
	"maps"
	"strings"

	"github.com/pawnkit/pawn-parser/token"
	"github.com/pawnkit/pawnkit-core/diagnostic"
)

func (e *engine) emitActive(f *frame) {
	tok := f.advance()
	if !f.currentActive() {
		return
	}
	//nolint:nestif // Macro expansion checks several ordered match conditions.
	if macroNameToken(tok, f.source) {
		name := macroPrefixText(tok.Text(f.source))
		if m, ok := e.macros.lookup(name); ok {
			if len(m.NamedParams) != 0 && f.cur().Kind == token.LParen {
				e.expandFunctionAt(f, tok, m)
				return
			}
			if e.expandPatternAt(f, tok, m) {
				return
			}
		}
	}
	e.appendOut(toPtok(f.source, tok, f.fileIndex))
}

// appendOut writes a token into the synthesized output buffer.
func (e *engine) appendOut(t ptok) {
	if e.truncated || e.pollCancellation() {
		return
	}
	e.declarations.observe(t)
	if e.listingCode >= 0 && e.listingCode < len(e.listing) {
		e.listing[e.listingCode].tokens = append(e.listing[e.listingCode].tokens, t)
	}
	e.outputTokens++
	if e.outputTokens > e.opts.MaxOutputTokens {
		e.truncated = true
		e.stopped = true
		e.diags = append(e.diags, Diagnostic{
			Code: CodeOutputSizeLimit, Severity: diagnostic.SeverityError,
			Message: "preprocessor output size limit exceeded",
			Range:   ByteRange{Start: t.Start.Offset, End: t.End.Offset},
		})
		return
	}
	if len(e.expandedBuf) > 0 {
		e.expandedBuf = append(e.expandedBuf, ' ')
	}
	start := len(e.expandedBuf)
	e.expandedBuf = append(e.expandedBuf, t.text...)
	out := t.Token
	if out.Origin == nil {
		out.Origin = &token.Origin{
			Span: token.Span{File: t.file, Start: out.Start, End: out.End},
		}
	}
	out.Start = token.Position{Offset: start}
	out.End = token.Position{Offset: start + len(t.text)}
	out.LeadingTrivia = nil
	out.TrailingTrivia = nil
	e.reserveOutputToken()
	e.out = append(e.out, out)
}

func (e *engine) reserveOutputToken() {
	if len(e.out) < cap(e.out) {
		return
	}
	limit := e.opts.MaxOutputTokens + 1
	capacity := min(max(64, cap(e.out)*2), limit)
	next := make([]token.Token, len(e.out), capacity)
	copy(next, e.out)
	e.out = next
}

func wrapOrigin(t ptok, inv token.Span, macroName string) ptok {
	parent := t.Origin
	if parent == nil {
		parent = &token.Origin{Span: token.Span{File: t.file, Start: t.Start, End: t.End}}
	}
	t.Origin = &token.Origin{Span: inv, Macro: macroName, Parent: parent}
	return t
}

func invocationSpan(file uint32, start, end token.Token) token.Span {
	return token.Span{File: file, Start: start.Start, End: end.End}
}

func nestedInvocationSpan(start, end ptok) token.Span {
	if start.Origin != nil {
		return start.Origin.Span
	}
	return token.Span{File: start.file, Start: start.Start, End: end.End}
}

func (e *engine) expandFunctionAt(f *frame, tok token.Token, m Macro) {
	f.advance() // '('
	args, closeParen, ok := e.collectArgs(f)
	if !ok {
		if f.currentActive() {
			e.diag(f, CodeUnterminatedInvocation, diagnostic.SeverityError,
				"unterminated invocation of macro '"+m.Name+"'", spanOf(tok, tok))
		}
		e.appendOut(toPtok(f.source, tok, f.fileIndex))
		return
	}
	if len(args) == 0 && m.ParamCount > 0 {
		args = [][]ptok{nil}
	}
	if len(args) > m.ParamCount && m.ParamCount > 0 {
		last := m.ParamCount - 1
		merged := append([]ptok(nil), args[last]...)
		comma := ptok{Token: token.Token{Kind: token.Comma, Start: closeParen.Start, End: closeParen.Start}, text: ",", file: f.fileIndex}
		for _, extra := range args[m.ParamCount:] {
			merged = append(merged, comma)
			merged = append(merged, extra...)
		}
		args = append(args[:last], merged)
	}
	if len(args) < m.ParamCount && !m.FlexiblePattern && f.currentActive() {
		e.diag(f, CodeMacroArgumentMismatch, diagnostic.SeverityWarning,
			"macro invocation argument count mismatch", spanOf(tok, closeParen))
	}
	inv := invocationSpan(f.fileIndex, tok, closeParen)
	e.recordInvocation(inv)
	body := substituteParams(m, args, inv)
	closing := toPtok(f.source, closeParen, f.fileIndex)
	e.expandWithFrameRemainder(f, body, closeParen, closing.trailing, hideSet{}.with(m.Name), 1)
}

// expandWithFrameRemainder rescans a top-level replacement together with the
// unconsumed tokens on its source line. Pawn macro replacement is textual: an
// object-like replacement may name a function-like macro whose opening
// parenthesis is still in the caller's token stream. Rescanning the replacement
// in isolation misses that boundary, which is a pattern used extensively by
// YSI's ALS chains.
func (e *engine) expandWithFrameRemainder(f *frame, replacement []ptok, consumed token.Token, boundaryTrailing string, hide hideSet, depth int) {
	consumedToken := toPtok(f.source, consumed, f.fileIndex)
	if len(replacement) != 0 {
		replacement[len(replacement)-1].trailing += boundaryTrailing
	}
	for index := range replacement {
		replacement[index].hide = mergeHideSets(replacement[index].hide, hide)
		if replacement[index].depth < depth {
			replacement[index].depth = depth
		}
	}

	combined := replacement
	if !endsLine(consumed) {
		for !f.atEnd() {
			item := f.advance()
			combined = append(combined, toPtok(f.source, item, f.fileIndex))
			if endsLine(item) {
				break
			}
		}
	}
	if len(replacement) == 0 && len(combined) != 0 {
		combined[0].leading = consumedToken.leading + boundaryTrailing + combined[0].leading
	}
	e.expandRun(f, combined, nil, 0)
}

func (e *engine) recordInvocation(inv token.Span) {
	e.invocations = append(e.invocations, MacroInvocation{
		File: inv.File,
		Range: ByteRange{
			Start: inv.Start.Offset,
			End:   inv.End.Offset,
		},
	})
	if e.listingCode >= 0 && e.listingCode < len(e.listing) {
		e.listing[e.listingCode].replacements = append(e.listing[e.listingCode].replacements, ByteRange{
			Start: inv.Start.Offset,
			End:   inv.End.Offset,
		})
	}
}

// collectArgs scans comma-separated argument token runs starting right
// after an already-consumed '(', respecting nested (), [], {} depth, up to
// and including the matching ')'. A single empty argument list "()"
// produces zero arguments.
func (e *engine) collectArgs(f *frame) (args [][]ptok, closeParen token.Token, ok bool) {
	if f.cur().Kind == token.RParen {
		return nil, f.advance(), true
	}
	depth := 0
	var current []ptok
	for {
		if e.pollCancellation() {
			return nil, token.Token{}, false
		}
		if f.atEnd() {
			return nil, token.Token{}, false
		}
		t := f.cur()
		//nolint:exhaustive // Argument collection only handles grouping tokens.
		switch t.Kind {
		case token.LParen, token.LBracket, token.LBrace:
			depth++
		case token.RBracket, token.RBrace:
			depth--
		case token.RParen:
			if depth == 0 {
				f.advance()
				args = append(args, current)
				return args, t, true
			}
			depth--
		case token.Comma:
			if depth == 0 {
				f.advance()
				args = append(args, current)
				current = nil
				continue
			}
		}
		item := toPtok(f.source, t, f.fileIndex)
		retainTokenOrigin(&item)
		current = append(current, item)
		f.advance()
	}
}

func substituteParams(m Macro, args [][]ptok, inv token.Span) []ptok {
	var out []ptok
	for _, bt := range m.Body {
		if bt.Kind == token.MacroParam {
			if idx, isParam := parseParamIndex(bt.text); isParam {
				slot, declared := m.ParamSlots[idx]
				if declared && slot < len(args) {
					for _, at := range args[slot] {
						out = append(out, wrapOrigin(at, inv, m.Name))
					}
				}
				continue
			}
			out = append(out, wrapOrigin(bt, inv, m.Name))
			continue
		}
		if bt.Kind == token.Identifier {
			if idx, named := m.NamedParams[bt.text]; named {
				if idx < len(args) {
					for _, at := range args[idx] {
						out = append(out, wrapOrigin(at, inv, m.Name))
					}
				}
				continue
			}
		}
		out = append(out, wrapOrigin(bt, inv, m.Name))
	}
	return out
}

//nolint:gocyclo // Expansion order and recursion guards are coupled.
func (e *engine) expandRun(f *frame, toks []ptok, hide hideSet, depth int) {
	for index := range toks {
		toks[index].hide = mergeHideSets(toks[index].hide, hide)
		if toks[index].depth < depth {
			toks[index].depth = depth
		}
	}
	i := 0
	for i < len(toks) {
		if e.truncated || e.pollCancellation() {
			break
		}
		if e.conditionMode && toks[i].Kind == token.KwDefined {
			e.appendOut(toks[i])
			i++
			if i < len(toks) && toks[i].Kind == token.LParen {
				e.appendOut(toks[i])
				i++
			}
			if i < len(toks) && toks[i].Kind == token.Identifier {
				name := macroPrefixText(toks[i].text)
				if name != "" {
					toks[i].hide = mergeHideSets(toks[i].hide, hideSet{}.with(name))
				}
			}
			continue
		}
		if i+1 < len(toks) {
			if pasted, ok := pasteAdjacentAlphanum(toks[i], toks[i+1]); ok {
				toks = spliceExpansion(toks, i, i+2, []ptok{pasted})
				continue
			}
		}
		t := toks[i]
		name := macroPrefixText(t.text)
		//nolint:nestif // Rescanning combines macro, pattern, and boundary checks.
		if name != "" && !t.hide[name] {
			if m, ok := e.macros.lookup(name); ok {
				var replacement []ptok
				next := i
				boundaryTrailing := ""
				matched := false
				if len(m.NamedParams) != 0 && i+1 < len(toks) && toks[i+1].Kind == token.LParen {
					args, endIdx, ok := collectArgsSlice(toks, i+2)
					if ok {
						inv := nestedInvocationSpan(t, toks[endIdx-1])
						replacement = substituteParams(m, args, inv)
						next = endIdx
						boundaryTrailing = toks[endIdx-1].trailing
						matched = true
					}
				}
				if !matched {
					replacement, next, boundaryTrailing, matched = e.expandPatternSlice(toks, i, m)
				}
				var signature string
				if matched && m.Pattern != m.Name {
					signature = expansionSignature(toks[i:next])
					if t.seen[m.Name] == signature {
						matched = false
					}
				}
				if matched {
					//nolint:gocritic // Replacement boundaries have distinct trivia rules.
					if len(replacement) != 0 {
						replacement[0].leading = t.leading + replacement[0].leading
					} else if next < len(toks) {
						toks[next].leading = t.leading + boundaryTrailing + toks[next].leading
					} else if i > 0 {
						toks[i-1].trailing += t.leading + boundaryTrailing
					}
					if next > i {
						invocation := expansionSpan(toks[i:next])
						if t.Origin == nil {
							e.recordInvocation(invocation)
						} else {
							e.extendInvocation(invocation)
						}
					}
					if len(replacement) != 0 && next > i {
						replacement[len(replacement)-1].trailing += boundaryTrailing
					}
					nextDepth := t.depth + 1
					if nextDepth > e.opts.MaxExpansionDepth {
						e.reportExpansionDepth(f, toks[i:next])
						e.appendOut(t)
						i++
						continue
					}
					// PawnCC rescans textually and permits a macro to become active
					// again after an intervening replacement changes its suffix. Keep
					// only the immediately expanded macro disabled on literal body
					// tokens; retaining the full ancestry stalls YSI chains such as
					// _ADDR@ -> _ADDR@i -> _ADDR@.
					var nextHide hideSet
					if m.Pattern == m.Name {
						nextHide = hideSet{}.with(m.Name)
					}
					for index := range replacement {
						replacement[index].hide = mergeHideSets(replacement[index].hide, nextHide)
						replacement[index].seen = extendExpansionHistory(replacement[index].seen, t.seen, m.Name, signature)
						replacement[index].depth = nextDepth
					}
					toks = spliceExpansion(toks, i, next, replacement)
					continue
				}
			}
		}
		e.appendOut(t)
		i++
	}
}

func expansionSignature(items []ptok) string {
	var signature strings.Builder
	for _, item := range items {
		signature.WriteString(item.text)
		signature.WriteString(item.trailing)
	}
	return signature.String()
}

func extendExpansionHistory(current, parent map[string]string, name, signature string) map[string]string {
	if signature == "" && len(current) == 0 && len(parent) == 0 {
		return nil
	}
	history := make(map[string]string, len(current)+len(parent)+1)
	maps.Copy(history, parent)
	maps.Copy(history, current)
	if signature != "" {
		history[name] = signature
	}
	return history
}

func pasteAdjacentAlphanum(left, right ptok) (ptok, bool) {
	if left.trailing != "" || right.leading != "" || left.text == "" || right.text == "" {
		return ptok{}, false
	}
	for index, char := range []byte(left.text + right.text) {
		if index == 0 {
			if !pawnAlpha(char) {
				return ptok{}, false
			}
		} else if !pawnAlphanum(char) {
			return ptok{}, false
		}
	}
	left.text += right.text
	left.End = right.End
	left.trailing = right.trailing
	left.TrailingTrivia = right.TrailingTrivia
	left.hide = mergeHideSets(left.hide, right.hide)
	left.depth = max(left.depth, right.depth)
	leftSpan := listingSpan(left)
	rightSpan := listingSpan(right)
	if leftSpan.File == rightSpan.File {
		leftSpan.Start.Offset = min(leftSpan.Start.Offset, rightSpan.Start.Offset)
		leftSpan.End.Offset = max(leftSpan.End.Offset, rightSpan.End.Offset)
		left.listed = &leftSpan
	}
	return left, true
}

func expansionSpan(items []ptok) token.Span {
	if len(items) == 0 {
		return token.Span{}
	}
	span := listingSpan(items[0])
	for _, item := range items[1:] {
		current := listingSpan(item)
		if current.File != span.File {
			continue
		}
		if current.Start.Offset < span.Start.Offset {
			span.Start = current.Start
		}
		if current.End.Offset > span.End.Offset {
			span.End = current.End
		}
	}
	return span
}

func (e *engine) extendInvocation(inv token.Span) {
	start, end := inv.Start.Offset, inv.End.Offset
	//nolint:modernize // This loop needs pointers to backing slice entries.
	for index := len(e.invocations) - 1; index >= 0; index-- {
		item := &e.invocations[index]
		if item.File != inv.File || item.Range.End < start || item.Range.Start > end {
			continue
		}
		item.Range.Start = min(item.Range.Start, start)
		item.Range.End = max(item.Range.End, end)
		break
	}
	if e.listingCode < 0 || e.listingCode >= len(e.listing) {
		return
	}
	replacements := e.listing[e.listingCode].replacements
	//nolint:modernize // This loop needs pointers to backing slice entries.
	for index := len(replacements) - 1; index >= 0; index-- {
		item := &replacements[index]
		if item.End < start || item.Start > end {
			continue
		}
		item.Start = min(item.Start, start)
		item.End = max(item.End, end)
		return
	}
}

func mergeHideSets(left, right hideSet) hideSet {
	if len(left) == 0 {
		return right
	}
	if len(right) == 0 {
		return left
	}
	merged := make(hideSet, len(left)+len(right))
	for name := range left {
		merged[name] = true
	}
	for name := range right {
		merged[name] = true
	}
	return merged
}

func spliceExpansion(tokens []ptok, start, end int, replacement []ptok) []ptok {
	result := make([]ptok, 0, len(tokens)-(end-start)+len(replacement))
	result = append(result, tokens[:start]...)
	result = append(result, replacement...)
	result = append(result, tokens[end:]...)
	return result
}

func (e *engine) reportExpansionDepth(f *frame, tokens []ptok) {
	e.truncated = true
	e.stopped = true
	if e.depthLimitWarned {
		return
	}
	e.depthLimitWarned = true
	r := ByteRange{}
	if len(tokens) != 0 {
		r = ByteRange{Start: tokens[0].Start.Offset, End: tokens[len(tokens)-1].End.Offset}
	}
	e.diags = append(e.diags, Diagnostic{
		File: f.fileIndex, Code: CodeExpansionDepthLimit, Severity: diagnostic.SeverityError,
		Message: "macro expansion depth limit exceeded", Range: r,
	})
}

// collectArgsSlice mirrors engine.collectArgs but operates over a detached
// token slice (used when rescanning an already-substituted macro body)
// rather than a live frame cursor.
func collectArgsSlice(toks []ptok, start int) (args [][]ptok, next int, ok bool) {
	i := start
	if i >= len(toks) {
		return nil, 0, false
	}
	if toks[i].Kind == token.RParen {
		return nil, i + 1, true
	}
	depth := 0
	var current []ptok
	for i < len(toks) {
		t := toks[i]
		//nolint:exhaustive // Argument collection only handles grouping tokens.
		switch t.Kind {
		case token.LParen, token.LBracket, token.LBrace:
			depth++
		case token.RBracket, token.RBrace:
			depth--
		case token.RParen:
			if depth == 0 {
				args = append(args, current)
				return args, i + 1, true
			}
			depth--
		case token.Comma:
			if depth == 0 {
				args = append(args, current)
				current = nil
				i++
				continue
			}
		}
		current = append(current, t)
		i++
	}
	return nil, 0, false
}
