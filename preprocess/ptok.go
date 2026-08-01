package preprocess

import "github.com/pawnkit/pawn-parser/token"

// ptok pairs a token with its spelled text. Text extraction from a bare
// token.Token requires knowing which source buffer its offsets index into;
// once tokens start moving between files (via #include splicing) or being
// copied during macro substitution, that buffer is no longer implicit, so
// the expansion pipeline carries text explicitly instead of re-deriving it
// from offsets that may point into a different file's bytes.
type ptok struct {
	token.Token
	text     string
	leading  string
	trailing string
	file     uint32
	hide     hideSet
	seen     map[string]string
	depth    int
	listed   *token.Span
}

func toPtok(source []byte, t token.Token, file ...uint32) ptok {
	var id uint32
	if len(file) != 0 {
		id = file[0]
	}
	var trailing string
	var leading string
	if len(t.LeadingTrivia) != 0 {
		start := t.LeadingTrivia[0].Start.Offset
		end := t.LeadingTrivia[len(t.LeadingTrivia)-1].End.Offset
		if start >= 0 && start <= end && end <= len(source) {
			leading = string(source[start:end])
		}
	}
	if len(t.TrailingTrivia) != 0 {
		start := t.TrailingTrivia[0].Start.Offset
		end := t.TrailingTrivia[len(t.TrailingTrivia)-1].End.Offset
		if start >= 0 && start <= end && end <= len(source) {
			trailing = string(source[start:end])
		}
	}
	return ptok{Token: t, text: t.Text(source), leading: leading, trailing: trailing, file: id}
}

func toPtoks(source []byte, toks []token.Token, file ...uint32) []ptok {
	out := make([]ptok, len(toks))
	for i, t := range toks {
		out[i] = toPtok(source, t, file...)
	}
	return out
}

func retainTokenOrigins(toks []ptok) []ptok {
	for i := range toks {
		retainTokenOrigin(&toks[i])
	}
	return toks
}

func retainTokenOrigin(tok *ptok) {
	if tok == nil || tok.Origin != nil {
		return
	}
	tok.Origin = &token.Origin{
		Span: token.Span{File: tok.file, Start: tok.Start, End: tok.End},
	}
}
