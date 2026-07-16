package token

import "sort"

// LineMap maps byte offsets to source positions.
type LineMap struct {
	Starts []uint32
}

// NewLineMap builds a line map for source.
func NewLineMap(source []byte) LineMap {
	starts := make([]uint32, 1, 128)
	for offset, value := range source {
		if value == '\n' {
			starts = append(starts, uint32(offset+1)) // #nosec G115 -- Source indexes fit parser offsets.
		}
	}
	return LineMap{Starts: starts}
}

// Position returns the source position at offset.
func (m LineMap) Position(offset uint32) Position {
	line := sort.Search(len(m.Starts), func(i int) bool { return m.Starts[i] > offset })
	if line == 0 {
		return Position{Offset: int(offset), Line: 1, Col: int(offset) + 1}
	}
	start := m.Starts[line-1]
	return Position{Offset: int(offset), Line: line, Col: int(offset-start) + 1}
}
