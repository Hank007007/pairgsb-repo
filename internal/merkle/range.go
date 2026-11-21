package merkle

// RangeRoot returns the Merkle root over the records whose sequence numbers
// fall inside [from, to].
//
// The leaves map is keyed by sequence number. Bounds beyond the available
// records are treated as if they were clipped to the records that exist. An
// interval that contains no records yields a zero-filled digest of the same
// size as a root.
func RangeRoot(leaves map[int64][]byte, from, to int64) []byte {
	if from > to {
		return make([]byte, 32)
	}
	ordered := make([][]byte, 0, len(leaves))
	for seq, h := range leaves {
		if seq < from || seq > to {
			continue
		}
		ordered = append(ordered, Leaf(h))
	}
	if len(ordered) == 0 {
		return make([]byte, 32)
	}
	return TreeRoot(ordered)
}
