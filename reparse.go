package parser

import (
	"bytes"

	"github.com/pawnkit/pawn-parser/token"
)

// ReparseCompactDeclaration reparses one changed top-level declaration.
func ReparseCompactDeclaration(
	source []byte,
	tokens []token.Token,
	previous *CompactFile,
	before ByteRange,
	after ByteRange,
) (*CompactFile, bool) {
	if previous == nil || previous.HasParseErrors() || len(previous.Lines.Starts) != 0 ||
		!validEdit(previous.Source, source, before, after) {
		return nil, false
	}

	index := BuildDeclarationIndex(previous)
	if !index.Reliable() {
		return nil, false
	}
	target, targetIndex, ok := containingDeclaration(index, before)
	if !ok {
		return nil, false
	}

	delta := (after.End - after.Start) - (before.End - before.Start)
	currentRange := ByteRange{Start: target.Range.Start, End: target.Range.End + delta}
	if currentRange.End < currentRange.Start || currentRange.End > len(source) ||
		after.Start < currentRange.Start || after.End > currentRange.End {
		return nil, false
	}

	fragment := ParseCompact(source[currentRange.Start:currentRange.End], ParseOptions{
		DiscardTokens: true,
		DiscardTrivia: true,
	})
	fragmentIndex := BuildDeclarationIndex(fragment)
	replacement, ok := fragmentIndex.At(0)
	if fragment.HasParseErrors() || !fragmentIndex.Reliable() || fragmentIndex.Len() != 1 || !ok ||
		replacement.Range.Start != 0 || replacement.Range.End != currentRange.End-currentRange.Start {
		return nil, false
	}

	oldRootChild, ok := declarationChild(previous, target)
	if !ok {
		return nil, false
	}
	fragmentRootChildren := fragment.Tree.ChildIndices(fragment.Tree.Root)
	if len(fragmentRootChildren) != 1 {
		return nil, false
	}

	compact, trivia, origins, macroNames := compactTokens(tokens, ParseOptions{})
	tree, ok := spliceCompactTree(
		previous,
		fragment,
		oldRootChild,
		fragmentRootChildren[0],
		targetIndex,
		currentRange.Start,
		before,
		after,
	)
	if !ok {
		return nil, false
	}
	return &CompactFile{
		Source: source, Tokens: compact, Trivia: trivia, Origins: origins, MacroNames: macroNames,
		Tree: tree,
	}, true
}

func validEdit(previous, current []byte, before, after ByteRange) bool {
	if before.Start < 0 || before.End < before.Start || before.End > len(previous) ||
		after.Start < 0 || after.End < after.Start || after.End > len(current) ||
		before.Start != after.Start {
		return false
	}
	return bytes.Equal(previous[:before.Start], current[:after.Start]) &&
		bytes.Equal(previous[before.End:], current[after.End:])
}

func containingDeclaration(index DeclarationIndex, edit ByteRange) (DeclarationBoundary, int, bool) {
	found := -1
	var target DeclarationBoundary
	for i := range index.Len() {
		item, _ := index.At(i)
		if edit.Start < item.Range.Start || edit.End > item.Range.End {
			continue
		}
		if found != -1 {
			return DeclarationBoundary{}, 0, false
		}
		found, target = i, item
	}
	return target, found, found != -1
}

func declarationChild(file *CompactFile, target DeclarationBoundary) (uint32, bool) {
	var found uint32
	ok := false
	for _, child := range file.Tree.ChildIndices(file.Tree.Root) {
		node := file.Tree.Nodes[child]
		if node.Range() != target.Range {
			continue
		}
		if ok {
			return 0, false
		}
		found, ok = child, true
	}
	return found, ok
}

type compactTreeCopier struct {
	tree        CompactTree
	old         *CompactTree
	fragment    *CompactTree
	oldMap      map[uint32]uint32
	fragmentMap map[uint32]uint32
	delta       int
	fragmentAt  int
	before      ByteRange
	after       ByteRange
}

func spliceCompactTree(
	previous, fragment *CompactFile,
	oldDeclaration, replacement uint32,
	declarationIndex, fragmentAt int,
	before, after ByteRange,
) (CompactTree, bool) {
	copier := compactTreeCopier{
		old: &previous.Tree, fragment: &fragment.Tree,
		oldMap: make(map[uint32]uint32), fragmentMap: make(map[uint32]uint32),
		delta:      after.End - after.Start - (before.End - before.Start),
		fragmentAt: fragmentAt, before: before, after: after,
	}
	root := previous.Tree.Nodes[previous.Tree.Root]
	rootChildren := previous.Tree.ChildIndices(previous.Tree.Root)
	children := make([]uint32, 0, len(rootChildren))
	for i, child := range rootChildren {
		var next uint32
		var ok bool
		if i == declarationIndex && child == oldDeclaration {
			next, ok = copier.copyFragment(replacement)
		} else {
			next, ok = copier.copyOld(child)
		}
		if !ok {
			return CompactTree{}, false
		}
		children = append(children, next)
	}
	root.Start, root.End = 0, compactUint(len(previous.Source)+copier.delta)
	root.ChildStart = compactUint(len(copier.tree.Children))
	root.ChildCount = compactUint(len(children))
	copier.tree.Children = append(copier.tree.Children, children...)
	root.FieldStart, root.FieldCount = compactUint(len(copier.tree.Fields)), 0
	copier.tree.Root = compactUint(len(copier.tree.Nodes))
	copier.tree.Nodes = append(copier.tree.Nodes, root)
	return copier.tree, true
}

func (c *compactTreeCopier) copyOld(index uint32) (uint32, bool) {
	return c.copy(index, c.old, c.oldMap, false)
}

func (c *compactTreeCopier) copyFragment(index uint32) (uint32, bool) {
	return c.copy(index, c.fragment, c.fragmentMap, true)
}

func (c *compactTreeCopier) copy(
	index uint32,
	from *CompactTree,
	mapping map[uint32]uint32,
	fragment bool,
) (uint32, bool) {
	if mapped, ok := mapping[index]; ok {
		return mapped, true
	}
	if int(index) >= len(from.Nodes) {
		return 0, false
	}
	node := from.Nodes[index]
	mapped := compactUint(len(c.tree.Nodes))
	mapping[index] = mapped
	c.tree.Nodes = append(c.tree.Nodes, CompactNode{})

	if fragment {
		shiftNode(&node, c.fragmentAt)
	} else {
		rebaseNode(&node, c.before, c.after)
	}

	children := make([]uint32, 0, node.ChildCount)
	for _, child := range from.ChildIndices(index) {
		next, ok := c.copy(child, from, mapping, fragment)
		if !ok {
			return 0, false
		}
		children = append(children, next)
	}
	fields := make([]CompactField, 0, node.FieldCount)
	for _, field := range from.Fields[node.FieldStart : node.FieldStart+node.FieldCount] {
		next, ok := c.copy(field.Node, from, mapping, fragment)
		if !ok {
			return 0, false
		}
		fields = append(fields, CompactField{ID: field.ID, Node: next})
	}
	node.ChildStart = compactUint(len(c.tree.Children))
	node.ChildCount = compactUint(len(children))
	c.tree.Children = append(c.tree.Children, children...)
	node.FieldStart = compactUint(len(c.tree.Fields))
	node.FieldCount = compactUint(len(fields))
	c.tree.Fields = append(c.tree.Fields, fields...)
	c.tree.Nodes[mapped] = node
	return mapped, true
}

func shiftNode(node *CompactNode, offset int) {
	node.Start += compactUint(offset)
	node.End += compactUint(offset)
	if node.TokenKind != token.Invalid {
		node.TokenStart += compactUint(offset)
		node.TokenEnd += compactUint(offset)
	}
}

func rebaseNode(node *CompactNode, before, after ByteRange) {
	node.Start = shiftedStart(node.Start, before, after)
	node.End = shiftedEnd(node.End, before, after)
	if node.TokenKind != token.Invalid {
		node.TokenStart = shiftedStart(node.TokenStart, before, after)
		node.TokenEnd = shiftedEnd(node.TokenEnd, before, after)
	}
}
