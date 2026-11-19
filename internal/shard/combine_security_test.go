package shard

import (
	"errors"
	"testing"
)

// TestCombineRejectsMixedGeneration reproduces the review finding: shards
// from two generations of the same split must never combine into a key.
func TestCombineRejectsMixedGeneration(t *testing.T) {
	secret := []byte("cross generation secret")

	gen1, err := Split(secret, 3, 2, "split-a", 1)
	if err != nil {
		t.Fatalf("Split gen1: %v", err)
	}
	gen2, err := Split(secret, 3, 2, "split-a", 2)
	if err != nil {
		t.Fatalf("Split gen2: %v", err)
	}

	mixed := []Shard{gen1[0], gen1[1], gen2[2]}
	got, err := Combine(mixed)
	if err == nil {
		t.Fatalf("Combine mixed generations succeeded, returned %x", got)
	}
	if !errors.Is(err, ErrMismatchedGen) {
		t.Fatalf("Combine mixed generations: err = %v, want ErrMismatchedGen", err)
	}
	if got != nil {
		t.Fatalf("Combine returned candidate key bytes on failure: %x", got)
	}
}

func TestCombineRejectsMixedSplitID(t *testing.T) {
	secret := []byte("cross split secret")
	first, err := Split(secret, 3, 2, "split-a", 1)
	if err != nil {
		t.Fatalf("Split first: %v", err)
	}
	second, err := Split(secret, 3, 2, "split-b", 1)
	if err != nil {
		t.Fatalf("Split second: %v", err)
	}

	mixed := []Shard{first[0], first[1], second[2]}
	if _, err := Combine(mixed); !errors.Is(err, ErrMismatchedSplitID) {
		t.Fatalf("Combine mixed split-ids: err = %v, want ErrMismatchedSplitID", err)
	}
}

func TestCombineRejectsInvalidBatchIdentity(t *testing.T) {
	base, err := Split([]byte("identity secret"), 3, 2, "split-a", 1)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}

	emptySplit := append([]Shard(nil), base...)
	emptySplit[0].SplitID = ""
	if _, err := Combine(emptySplit); !errors.Is(err, ErrEmptyShardSplitID) {
		t.Fatalf("Combine with empty split-id: err = %v, want ErrEmptyShardSplitID", err)
	}

	negativeGen := append([]Shard(nil), base...)
	negativeGen[0].Generation = -1
	if _, err := Combine(negativeGen); !errors.Is(err, ErrNegativeGeneration) {
		t.Fatalf("Combine with negative generation: err = %v, want ErrNegativeGeneration", err)
	}
}

// TestCombineBatchCheckRunsBeforeReconstruction ensures a mixed batch that is
// also threshold-sufficient is still rejected instead of producing output.
func TestCombineBatchCheckRunsBeforeReconstruction(t *testing.T) {
	secret := []byte("ordering check secret")
	gen1, err := Split(secret, 4, 3, "split-a", 1)
	if err != nil {
		t.Fatalf("Split gen1: %v", err)
	}
	gen2, err := Split(secret, 4, 3, "split-a", 2)
	if err != nil {
		t.Fatalf("Split gen2: %v", err)
	}

	mixed := []Shard{gen1[0], gen1[1], gen2[2]}
	if _, err := Combine(mixed); err == nil {
		t.Fatal("threshold-sufficient mixed batch was reconstructed")
	}
}
