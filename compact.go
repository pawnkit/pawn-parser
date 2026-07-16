package parser

import "github.com/pawnkit/pawn-parser/token"

// CompactFile is an index-based CST.
type CompactFile struct {
	Source      []byte
	Tokens      []token.Token
	Tree        CompactTree
	Broken      bool
	Diagnostics []Diagnostic
}

// HasParseErrors reports whether compact parsing produced a syntax error.
func (f *CompactFile) HasParseErrors() bool {
	return f == nil || len(f.Tree.Nodes) == 0 || f.Broken ||
		f.Tree.Nodes[f.Tree.Root].HasError || len(f.Diagnostics) != 0
}

// CompactTree stores nodes and edges in arrays.
type CompactTree struct {
	Nodes    []CompactNode
	Children []int
	Fields   []CompactField
	Root     int
}

// CompactNode stores one tree node.
type CompactNode struct {
	Kind Kind

	Start int
	End   int

	TokenKind  token.Kind
	TokenStart int
	TokenEnd   int

	ChildStart int
	ChildCount int
	FieldStart int
	FieldCount int

	HasError    bool
	MissingSemi bool
}

// Text returns the node's exact source text.
func (n CompactNode) Text(source []byte) string {
	if n.Start < 0 || n.End > len(source) || n.Start > n.End {
		return ""
	}
	return string(source[n.Start:n.End])
}

// CompactField maps a field name to a node.
type CompactField struct {
	Name string
	Node int
}

// ChildIndices returns the child indices for node.
func (t *CompactTree) ChildIndices(node int) []int {
	if t == nil || node < 0 || node >= len(t.Nodes) {
		return nil
	}
	n := t.Nodes[node]
	return t.Children[n.ChildStart : n.ChildStart+n.ChildCount]
}

// Field returns the node index associated with name.
func (t *CompactTree) Field(node int, name string) (int, bool) {
	if t == nil || node < 0 || node >= len(t.Nodes) {
		return 0, false
	}
	n := t.Nodes[node]
	for _, field := range t.Fields[n.FieldStart : n.FieldStart+n.FieldCount] {
		if field.Name == name {
			return field.Node, true
		}
	}
	return 0, false
}

// ParseCompact parses source into a compact CST.
func ParseCompact(source []byte, options ParseOptions) *CompactFile {
	parsed := Parse(source)
	return compactFile(parsed, options)
}

// ParseTokensCompact parses tokens into a compact CST.
func ParseTokensCompact(source []byte, toks []token.Token, options ParseOptions) *CompactFile {
	parsed := ParseTokens(source, toks)
	return compactFile(parsed, options)
}

func compactFile(parsed *File, options ParseOptions) *CompactFile {
	tree := compactTree(parsed.Root)
	return &CompactFile{
		Source: parsed.Source, Tokens: retainedTokens(parsed.Tokens, options), Tree: tree,
		Broken: parsed.Broken, Diagnostics: parsed.Diagnostics,
	}
}

func compactTree(root *Node) CompactTree {
	if root == nil {
		return CompactTree{}
	}
	nodeCount, childCount, fieldCount := compactCounts(root)
	tree := CompactTree{
		Nodes:    make([]CompactNode, 0, nodeCount),
		Children: make([]int, 0, childCount),
		Fields:   make([]CompactField, 0, fieldCount),
	}
	var add func(*Node) int
	add = func(node *Node) int {
		index := len(tree.Nodes)
		tree.Nodes = append(tree.Nodes, CompactNode{
			Kind: node.Kind, Start: node.Start, End: node.End,
			TokenKind: node.Tok.Kind, TokenStart: node.Tok.Start.Offset, TokenEnd: node.Tok.End.Offset,
			HasError: node.HasError, MissingSemi: node.MissingSemi,
		})

		childStart := len(tree.Children)
		tree.Children = append(tree.Children, make([]int, len(node.Children))...)
		for i, child := range node.Children {
			tree.Children[childStart+i] = add(child)
		}

		fieldStart := len(tree.Fields)
		if node.fieldData != nil {
			appendField := func(field fieldEntry) {
				for childOffset, child := range node.Children {
					if child == field.node {
						tree.Fields = append(tree.Fields, CompactField{
							Name: field.name, Node: tree.Children[childStart+childOffset],
						})
						return
					}
				}
			}
			inlineCount := min(node.fieldData.count, len(node.fieldData.inline))
			for _, field := range node.fieldData.inline[:inlineCount] {
				appendField(field)
			}
			for _, field := range node.fieldData.spill {
				appendField(field)
			}
		}

		record := &tree.Nodes[index]
		record.ChildStart = childStart
		record.ChildCount = len(node.Children)
		record.FieldStart = fieldStart
		record.FieldCount = len(tree.Fields) - fieldStart
		return index
	}
	tree.Root = add(root)
	return tree
}

func compactCounts(node *Node) (nodes, children, fields int) {
	nodes = 1
	children = len(node.Children)
	if node.fieldData != nil {
		fields = node.fieldData.count
	}
	for _, child := range node.Children {
		childNodes, childChildren, childFields := compactCounts(child)
		nodes += childNodes
		children += childChildren
		fields += childFields
	}
	return nodes, children, fields
}
