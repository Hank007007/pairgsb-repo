package merkle

import (
	"bytes"
	"testing"
)

func TestTreeRootSingleLeaf(t *testing.T) {
	leaf := Leaf([]byte("record-hash"))
	root := TreeRoot([][]byte{leaf})
	if !bytes.Equal(root, leaf) {
		t.Fatal("a single-leaf tree should reduce to its leaf")
	}
}

func TestTreeRootIsDeterministic(t *testing.T) {
	leaves := [][]byte{
		Leaf([]byte("a")),
		Leaf([]byte("b")),
		Leaf([]byte("c")),
	}
	first := TreeRoot(leaves)
	second := TreeRoot(leaves)
	if !bytes.Equal(first, second) {
		t.Fatal("the same leaves produced two different roots")
	}
	if len(first) != 32 {
		t.Fatalf("root has %d bytes, want 32", len(first))
	}
}

func TestLeafAndNodeAreDomainSeparated(t *testing.T) {
	inner := []byte("x")
	if bytes.Equal(Leaf(inner), Node(inner, inner)) {
		t.Fatal("a leaf hash equals an inner-node hash")
	}
}

func TestRangeRootSingleRecord(t *testing.T) {
	leaves := map[int64][]byte{1: []byte("only")}
	got := RangeRoot(leaves, 1, 1)
	want := TreeRoot([][]byte{Leaf([]byte("only"))})
	if !bytes.Equal(got, want) {
		t.Fatal("range root over one record does not match its leaf")
	}
}
