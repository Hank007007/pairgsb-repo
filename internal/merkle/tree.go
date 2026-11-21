// Package merkle builds Merkle roots over chain records.
package merkle

import (
	"crypto/sha256"
)

// Leaf hashes a chain record hash into a Merkle leaf.
//
// Leaves and inner nodes use domain-separated prefixes so a leaf can never be
// mistaken for an inner node.
func Leaf(recordHash []byte) []byte {
	h := sha256.New()
	h.Write([]byte{0x00})
	h.Write(recordHash)
	return h.Sum(nil)
}

// Node hashes two child nodes together.
func Node(left, right []byte) []byte {
	h := sha256.New()
	h.Write([]byte{0x01})
	h.Write(left)
	h.Write(right)
	return h.Sum(nil)
}

// TreeRoot reduces an ordered list of leaves to a single root.
//
// Adjacent nodes are paired left-to-right. When a level holds an odd number
// of nodes the final node is paired with itself.
func TreeRoot(leaves [][]byte) []byte {
	if len(leaves) == 0 {
		return nil
	}
	level := make([][]byte, len(leaves))
	copy(level, leaves)
	for len(level) > 1 {
		next := make([][]byte, 0, (len(level)+1)/2)
		for i := 0; i < len(level); i += 2 {
			left := level[i]
			right := left
			if i+1 < len(level) {
				right = level[i+1]
			}
			next = append(next, Node(left, right))
		}
		level = next
	}
	return level[0]
}
