package chain

import "witnesschain/internal/merkle"

// newMerkleTree builds a Merkle tree over record hashes.
func newMerkleTree(hashes []string) (*merkle.Tree, error) {
	return merkle.New(hashes)
}
