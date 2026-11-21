package merkle

import (
	"fmt"
	"testing"
)

func sampleLeaves(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%064x", i+1)
	}
	return out
}

func TestNewRejectsEmpty(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("expected an error for an empty leaf set")
	}
}

func TestCountAndRootAreStable(t *testing.T) {
	leaves := sampleLeaves(5)
	t1, err := New(leaves)
	if err != nil {
		t.Fatal(err)
	}
	t2, err := New(leaves)
	if err != nil {
		t.Fatal(err)
	}
	if t1.Count() != 5 {
		t.Fatalf("Count = %d, want 5", t1.Count())
	}
	if t1.Root() != t2.Root() {
		t.Fatal("root is not deterministic for identical leaves")
	}
	if len(t1.Root()) != 64 {
		t.Fatalf("root length = %d, want 64", len(t1.Root()))
	}
}

func TestRootChangesWithLeaves(t *testing.T) {
	a, err := New(sampleLeaves(4))
	if err != nil {
		t.Fatal(err)
	}
	leaves := sampleLeaves(4)
	leaves[2] = fmt.Sprintf("%064x", 999)
	b, err := New(leaves)
	if err != nil {
		t.Fatal(err)
	}
	if a.Root() == b.Root() {
		t.Fatal("root did not change when a leaf changed")
	}
}

func TestProofForFirstLeafVerifies(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4, 5, 8, 16} {
		leaves := sampleLeaves(n)
		tree, err := New(leaves)
		if err != nil {
			t.Fatal(err)
		}
		proof, err := tree.Proof(0)
		if err != nil {
			t.Fatal(err)
		}
		if !VerifyProof(leaves[0], 0, proof, tree.Count(), tree.Root()) {
			t.Fatalf("proof for leaf 0 of %d did not verify", n)
		}
	}
}

func TestProofForSingleLeaf(t *testing.T) {
	leaves := sampleLeaves(1)
	tree, err := New(leaves)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := tree.Proof(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(proof) != 0 {
		t.Fatalf("single-leaf proof has %d steps, want 0", len(proof))
	}
	if !VerifyProof(leaves[0], 0, proof, 1, tree.Root()) {
		t.Fatal("single-leaf proof did not verify")
	}
}

func TestProofRejectsWrongLeaf(t *testing.T) {
	leaves := sampleLeaves(8)
	tree, err := New(leaves)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := tree.Proof(0)
	if err != nil {
		t.Fatal(err)
	}
	other := fmt.Sprintf("%064x", 12345)
	if VerifyProof(other, 0, proof, tree.Count(), tree.Root()) {
		t.Fatal("proof verified for a leaf that is not in the tree")
	}
}

func TestProofIndexRange(t *testing.T) {
	tree, err := New(sampleLeaves(4))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tree.Proof(-1); err == nil {
		t.Fatal("expected an error for a negative index")
	}
	if _, err := tree.Proof(4); err == nil {
		t.Fatal("expected an error for an index past the end")
	}
}

func TestProofOfOddTreeFirstLeaf(t *testing.T) {
	leaves := sampleLeaves(3)
	tree, err := New(leaves)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := tree.Proof(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(proof) != 2 {
		t.Fatalf("proof length = %d, want 2", len(proof))
	}
	if !VerifyProof(leaves[0], 0, proof, 3, tree.Root()) {
		t.Fatal("proof for the first leaf of a three-leaf tree did not verify")
	}
}
