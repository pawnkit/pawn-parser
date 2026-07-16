package parser

import "testing"

func TestNodeArenaPointersRemainStableAcrossGrowth(t *testing.T) {
	t.Parallel()

	var arena nodeArena
	first := arena.alloc()
	first.Kind = KindIdentifier

	for range initialNodeBlockSize + maxNodeBlockSize*2 {
		arena.alloc()
	}

	if first.Kind != KindIdentifier {
		t.Fatalf("first node changed after arena growth: got %v", first.Kind)
	}
	if first != &arena.blocks[0][0] {
		t.Fatal("first node pointer changed after arena growth")
	}
}

func TestNodeArenaUsesConservativeBoundedBlocks(t *testing.T) {
	t.Parallel()

	var arena nodeArena
	arena.alloc()
	if got := len(arena.blocks[0]); got != initialNodeBlockSize {
		t.Fatalf("initial block size = %d, want %d", got, initialNodeBlockSize)
	}

	for range maxNodeBlockSize * 3 {
		arena.alloc()
	}
	for i, block := range arena.blocks {
		if len(block) > maxNodeBlockSize {
			t.Fatalf("block %d size = %d, exceeds maximum %d", i, len(block), maxNodeBlockSize)
		}
	}
}
