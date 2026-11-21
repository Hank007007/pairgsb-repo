// Package merkle builds Merkle trees over a segment's record hashes and
// produces/checks inclusion proofs.
//
// The normative description of leaf hashing, parent hashing, tree shape, root
// binding, and proof shape is in docs/chain-format.md §6.
package merkle

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

const (
	leafPrefix   = 0x00
	parentPrefix = 0x01
)

// ErrEmptyTree is returned when a tree is requested over zero leaves.
var ErrEmptyTree = errors.New("merkle: empty tree has no root")

// Step is one level of an inclusion proof.
//
// When Sibling is non-empty, it names the sibling node at this level and Left
// says whether that sibling is the left child. When Sibling is empty, the step
// is a promotion: the node was the unpaired final node of an odd level and was
// carried up unchanged.
type Step struct {
	Sibling string `json:"sibling,omitempty"`
	Left    bool   `json:"left,omitempty"`
}

// Tree is an immutable Merkle tree over a list of record hashes.
type Tree struct {
	leaves []string
	levels [][]string
}

// New builds a tree from record hashes (lowercase hex, 64 characters).
func New(recordHashes []string) (*Tree, error) {
	if len(recordHashes) == 0 {
		return nil, ErrEmptyTree
	}
	t := &Tree{leaves: append([]string(nil), recordHashes...)}
	level := make([]string, len(t.leaves))
	for i, h := range t.leaves {
		level[i] = leafHash(h)
	}
	t.levels = append(t.levels, level)
	for len(level) > 1 {
		next := make([]string, 0, (len(level)+1)/2)
		for i := 0; i < len(level); i += 2 {
			if i+1 >= len(level) {
				next = append(next, level[i])
				continue
			}
			next = append(next, parentHash(level[i], level[i+1]))
		}
		level = next
		t.levels = append(t.levels, level)
	}
	return t, nil
}

// Count returns the number of leaves.
func (t *Tree) Count() int { return len(t.leaves) }

// Top returns the node remaining after combining all levels.
func (t *Tree) Top() string {
	return t.levels[len(t.levels)-1][0]
}

// Root returns the Merkle root for the tree.
func (t *Tree) Root() string {
	return t.Top()
}

// Proof returns the inclusion proof for the leaf at index i.
func (t *Tree) Proof(i int) ([]Step, error) {
	if i < 0 || i >= len(t.leaves) {
		return nil, fmt.Errorf("merkle: leaf index %d out of range [0,%d)", i, len(t.leaves))
	}
	var proof []Step
	idx := i
	for depth := 0; depth < len(t.levels)-1; depth++ {
		level := t.levels[depth]
		var sib int
		if idx%2 == 0 {
			sib = idx + 1
		} else {
			sib = idx - 1
		}
		if sib < len(level) {
			proof = append(proof, Step{Sibling: level[sib], Left: idx%2 == 1})
		} else {
			proof = append(proof, Step{})
		}
		idx /= 2
	}
	return proof, nil
}

// leafHash computes SHA-256(0x00 || lowercase_hex(recordHash)).
func leafHash(recordHash string) string {
	h := sha256.New()
	h.Write([]byte{leafPrefix})
	h.Write([]byte(recordHash))
	return hex.EncodeToString(h.Sum(nil))
}

// parentHash computes SHA-256(0x01 || lowercase_hex(left) || lowercase_hex(right)).
func parentHash(left, right string) string {
	h := sha256.New()
	h.Write([]byte{parentPrefix})
	h.Write([]byte(left))
	h.Write([]byte(right))
	return hex.EncodeToString(h.Sum(nil))
}

// Combine folds a leaf and its proof into a candidate root node.
func Combine(leaf string, proof []Step) string {
	cur := leaf
	for _, step := range proof {
		if step.Sibling == "" {
			continue
		}
		cur = parentHash(cur, step.Sibling)
	}
	return cur
}

// VerifyProof reports whether the leaf at index belongs to a tree of count
// leaves whose root is root.
func VerifyProof(recordHash string, index int, proof []Step, count int, root string) bool {
	if count < 1 || recordHash == "" || root == "" {
		return false
	}
	cur := leafHash(recordHash)
	return Combine(cur, proof) == root
}
