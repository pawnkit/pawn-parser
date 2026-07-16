package parser

func (p *parser) parseSourceFile() *Node {
	root := p.storeNode(Node{Kind: KindSourceFile})
	items := p.parseItemSequence(itemGrammar{
		parseItem:       func(p *parser) *Node { return p.parseDeclaration() },
		stop:            func(p *parser) bool { return p.atEnd() },
		recoveryContext: "declaration",
	})
	for _, it := range items {
		p.addChild(root, it)
	}
	if len(p.toks) > 0 {
		root.End = p.toks[len(p.toks)-1].End.Offset
	}
	root.Trailing = p.toks[len(p.toks)-1].LeadingTrivia
	return root
}
