package merkle

// RangeRoot returns the Merkle root over the records whose sequence numbers
// fall inside [from, to].
//
// The leaves map is keyed by sequence number. The interval must be valid and
// completely covered by the map; inverted bounds, out-of-range bounds, and
// intervals with missing records return nil. A nil result is an error, not an
// empty root: an interval with no records has no root at all.
func RangeRoot(leaves map[int64][]byte, from, to int64) []byte {
	if from < 1 || to < from {
		return nil
	}
	count := to - from + 1
	if count <= 0 || count > int64(len(leaves)) {
		return nil
	}
	ordered := make([][]byte, 0, int(count))
	for seq := from; seq <= to; seq++ {
		h, ok := leaves[seq]
		if !ok {
			return nil
		}
		ordered = append(ordered, Leaf(h))
	}
	return TreeRoot(ordered)
}
