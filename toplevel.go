package parser

func (p *parser[N, S]) parseSourceFile() N {
	root := p.sink.Store(Node{Kind: KindSourceFile})
	items := p.parseItemSequence(itemGrammar[N, S]{
		parseMode:       itemParseDeclaration,
		recoveryContext: "declaration",
	})
	for _, it := range items {
		p.sink.AddChild(root, it)
	}
	if len(p.toks) > 0 {
		p.sink.SetEnd(root, p.toks[len(p.toks)-1].End.Offset)
	}
	p.sink.SetTrailing(root, p.toks[len(p.toks)-1].LeadingTrivia)
	return root
}
