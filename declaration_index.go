package parser

import (
	"crypto/sha256"
	"encoding/binary"
)

// DeclarationBoundary identifies one top-level declaration.
type DeclarationBoundary struct {
	Kind        Kind
	Range       ByteRange
	Name        string
	Identity    [sha256.Size]byte
	Fingerprint [sha256.Size]byte
	HasError    bool
}

// DeclarationIndex is an immutable declaration boundary table.
type DeclarationIndex struct {
	items    []DeclarationBoundary
	reliable bool
}

// BuildDeclarationIndex indexes the top-level declarations in file.
func BuildDeclarationIndex(file *CompactFile) DeclarationIndex {
	if file == nil {
		return DeclarationIndex{}
	}

	index := DeclarationIndex{reliable: !file.HasParseErrors()}
	root := file.Syntax()
	if !root.Valid() {
		return index
	}

	ordinals := make(map[declarationKey]uint32)
	declarations := root.Declarations()
	previousEnd := 0
	for declarations.Next() {
		node := declarations.Declaration()
		item := declarationBoundary(node, ordinals)
		if item.Range.Start < previousEnd || item.Range.End < item.Range.Start {
			index.reliable = false
		}
		if item.HasError {
			index.reliable = false
		}
		previousEnd = item.Range.End
		index.items = append(index.items, item)
	}

	return index
}

// RebaseDeclarationIndex updates one changed declaration without rehashing the
// rest of the file. It returns false when the edit changes declaration shape.
func RebaseDeclarationIndex(
	previous DeclarationIndex,
	current *CompactFile,
	before, after ByteRange,
) (DeclarationIndex, bool) {
	if !rebaseInputIsValid(previous, current, before, after) {
		return DeclarationIndex{}, false
	}
	target, ok := rebaseTarget(previous, before)
	if !ok {
		return DeclarationIndex{}, false
	}
	root := current.Syntax()
	if !root.Valid() {
		return DeclarationIndex{}, false
	}
	result := DeclarationIndex{items: make([]DeclarationBoundary, 0, previous.Len()), reliable: true}
	ordinals := make(map[declarationKey]uint32)
	declarations := root.Declarations()
	for index := 0; declarations.Next(); index++ {
		item, ok := rebaseBoundary(previous, index, target, declarations.Declaration(), ordinals)
		if !ok {
			return DeclarationIndex{}, false
		}
		result.items = append(result.items, item)
	}
	if len(result.items) != previous.Len() {
		return DeclarationIndex{}, false
	}
	return result, true
}

func rebaseInputIsValid(previous DeclarationIndex, current *CompactFile, before, after ByteRange) bool {
	return current != nil && previous.Reliable() && !current.HasParseErrors() &&
		validByteRange(before) && validByteRange(after)
}

func validByteRange(r ByteRange) bool {
	return r.Start >= 0 && r.End >= r.Start
}

func rebaseTarget(previous DeclarationIndex, before ByteRange) (int, bool) {
	target := -1
	for index, item := range previous.items {
		if !rangeContains(item.Range, before) {
			continue
		}
		if target >= 0 {
			return 0, false
		}
		target = index
	}
	return target, target >= 0
}

func rangeContains(outer, inner ByteRange) bool {
	if inner.Start == inner.End {
		return inner.Start >= outer.Start && inner.Start <= outer.End
	}
	return inner.Start >= outer.Start && inner.End <= outer.End
}

func rebaseBoundary(
	previous DeclarationIndex,
	index, target int,
	node SyntaxNode,
	ordinals map[declarationKey]uint32,
) (DeclarationBoundary, bool) {
	old, ok := previous.At(index)
	if !ok || old.Kind != node.Kind() || old.Name != declarationName(node) || node.HasError() {
		return DeclarationBoundary{}, false
	}
	key := declarationKey{kind: old.Kind, name: old.Name}
	item := old
	item.Range = node.Range()
	if index == target {
		item = declarationBoundary(node, ordinals)
		if item.Identity != old.Identity {
			return DeclarationBoundary{}, false
		}
	} else {
		ordinals[key]++
	}
	if item.Range.Start < 0 || item.Range.End < item.Range.Start {
		return DeclarationBoundary{}, false
	}
	return item, true
}

// Len returns the number of declarations.
func (i DeclarationIndex) Len() int { return len(i.items) }

// At returns the declaration at position n.
func (i DeclarationIndex) At(n int) (DeclarationBoundary, bool) {
	if n < 0 || n >= len(i.items) {
		return DeclarationBoundary{}, false
	}
	return i.items[n], true
}

// Reliable reports whether every boundary was recovered cleanly.
func (i DeclarationIndex) Reliable() bool { return i.reliable }

type declarationKey struct {
	kind Kind
	name string
}

func declarationBoundary(node SyntaxNode, ordinals map[declarationKey]uint32) DeclarationBoundary {
	name := declarationName(node)
	key := declarationKey{kind: node.Kind(), name: name}
	ordinal := ordinals[key]
	ordinals[key] = ordinal + 1

	item := DeclarationBoundary{
		Kind:        node.Kind(),
		Range:       node.Range(),
		Name:        name,
		Fingerprint: sha256.Sum256(node.Bytes()),
		HasError:    node.HasError(),
	}

	hash := sha256.New()
	var encoded [8]byte
	binary.LittleEndian.PutUint32(encoded[:4], uint32(item.Kind))
	binary.LittleEndian.PutUint32(encoded[4:], ordinal)
	_, _ = hash.Write(encoded[:])
	_, _ = hash.Write([]byte(name))
	copy(item.Identity[:], hash.Sum(nil))
	return item
}

func declarationName(node SyntaxNode) string {
	if name, ok := node.Field("name"); ok {
		return name.Token().Text()
	}
	if node.Kind() != KindVariableDeclaration {
		return ""
	}
	children := node.Children()
	for children.Next() {
		child := children.Node()
		if child.Kind() != KindVariableDeclarator {
			continue
		}
		if name, ok := child.Field("name"); ok {
			return name.Token().Text()
		}
	}
	return ""
}
