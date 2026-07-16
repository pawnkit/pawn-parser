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
	if p.toks.len() > 0 {
		p.sink.SetEnd(root, p.toks.endOffset(p.toks.len()-1))
	}
	p.sink.SetTrailing(root, p.toks.at(p.toks.len()-1).LeadingTrivia)
	return root
}
