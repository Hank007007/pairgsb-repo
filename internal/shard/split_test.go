package shard

import (
	"bytes"
	"testing"
)

func TestSplitAndCombineRoundTrip(t *testing.T) {
	secret := []byte("a master key of reasonable length")
	shards, err := Split(secret, 5, 3, "split-alpha", 7)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if len(shards) != 5 {
		t.Fatalf("got %d shards, want 5", len(shards))
	}

	for _, s := range shards {
		if s.SplitID != "split-alpha" {
			t.Errorf("shard %d: split id = %q", s.Index, s.SplitID)
		}
		if s.Generation != 7 {
			t.Errorf("shard %d: generation = %d", s.Index, s.Generation)
		}
		if s.Threshold != 3 {
			t.Errorf("shard %d: threshold = %d", s.Index, s.Threshold)
		}
		if len(s.Data) != len(secret) {
			t.Errorf("shard %d: payload length = %d", s.Index, len(s.Data))
		}
	}

	cases := [][]int{
		{0, 1, 2},
		{0, 2, 4},
		{1, 3, 4},
		{0, 1, 2, 3, 4},
	}
	for _, idx := range cases {
		picked := make([]Shard, len(idx))
		for i, j := range idx {
			picked[i] = shards[j]
		}
		got, err := Combine(picked)
		if err != nil {
			t.Fatalf("Combine(%v): %v", idx, err)
		}
		if !bytes.Equal(got, secret) {
			t.Fatalf("Combine(%v) = %x, want %x", idx, got, secret)
		}
	}
}

func TestCombineRequiresThreshold(t *testing.T) {
	shards, err := Split([]byte("another secret"), 5, 3, "split-beta", 1)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if _, err := Combine(shards[:2]); err == nil {
		t.Fatal("Combine accepted fewer shards than the threshold")
	}
}

func TestCombineRejectsDuplicateIndex(t *testing.T) {
	shards, err := Split([]byte("dup secret"), 4, 2, "split-gamma", 1)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if _, err := Combine([]Shard{shards[0], shards[0]}); err == nil {
		t.Fatal("Combine accepted duplicate indices")
	}
}

func TestCombineRejectsEmptyInput(t *testing.T) {
	if _, err := Combine(nil); err == nil {
		t.Fatal("Combine accepted no shards")
	}
}

func TestSplitRejectsBadParameters(t *testing.T) {
	if _, err := Split(nil, 3, 2, "s", 0); err == nil {
		t.Error("Split accepted an empty secret")
	}
	if _, err := Split([]byte("x"), 3, 4, "s", 0); err == nil {
		t.Error("Split accepted threshold above total")
	}
	if _, err := Split([]byte("x"), 3, 2, "", 0); err == nil {
		t.Error("Split accepted an empty split id")
	}
	if _, err := Split([]byte("x"), 3, 2, "s", -1); err == nil {
		t.Error("Split accepted a negative generation")
	}
}

func TestSplitUsesFreshRandomness(t *testing.T) {
	secret := []byte("repeatable input")
	first, err := Split(secret, 3, 2, "split-delta", 1)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	second, err := Split(secret, 3, 2, "split-delta", 1)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if bytes.Equal(first[0].Data, second[0].Data) {
		t.Fatal("two splits of the same secret produced identical shard payloads")
	}
}
