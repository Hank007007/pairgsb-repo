package chain

import (
	"path/filepath"
	"testing"
	"time"
)

func fixedClock(start time.Time) func() time.Time {
	t := start
	return func() time.Time {
		t = t.Add(1500 * time.Millisecond)
		return t
	}
}

func TestAppendBuildsLinkedChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	store, err := Open(path, WithClock(fixedClock(time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC))))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	for i := 0; i < 5; i++ {
		if _, err := store.Append("alice", "login", "session"); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	recs := store.Records()
	if len(recs) != 5 {
		t.Fatalf("got %d records, want 5", len(recs))
	}
	if recs[0].Prev != "" {
		t.Errorf("first record has prev %q, want empty", recs[0].Prev)
	}
	for i := 1; i < len(recs); i++ {
		if recs[i].Seq != int64(i)+1 {
			t.Errorf("record %d has seq %d", i, recs[i].Seq)
		}
		if recs[i].Prev != recs[i-1].Hash {
			t.Errorf("record %d prev does not link to record %d", i, i-1)
		}
	}
	if rep := Verify(recs, nil); !rep.OK() {
		t.Fatalf("verify reported problems: %v", rep.Problems)
	}
}

func TestReopenPreservesHashChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	store, err := Open(path, WithClock(fixedClock(time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC))))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := store.Append("alice", "read", "doc"); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	store.Close()

	again, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer again.Close()

	recs := again.Records()
	if len(recs) != 3 {
		t.Fatalf("got %d records after reopen, want 3", len(recs))
	}
	if rep := Verify(recs, nil); !rep.OK() {
		t.Fatalf("verify after reopen: %v", rep.Problems)
	}
}

func TestTamperedRecordFailsHashCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	store, err := Open(path, WithClock(fixedClock(time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC))))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()
	for i := 0; i < 3; i++ {
		if _, err := store.Append("alice", "read", "doc"); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	recs := store.Records()
	recs[1].Body = "rewritten"
	rep := Verify(recs, nil)
	if rep.OK() {
		t.Fatal("verify accepted a rewritten record")
	}
	found := false
	for _, p := range rep.Problems {
		if p.Seq == 2 && p.Field == "hash" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no hash problem for seq 2: %v", rep.Problems)
	}
}

func TestRangeSelectsBySeq(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	store, err := Open(path, WithClock(fixedClock(time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC))))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()
	for i := 0; i < 10; i++ {
		if _, err := store.Append("alice", "read", "doc"); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	got := store.Range(3, 6)
	if len(got) != 4 || got[0].Seq != 3 || got[3].Seq != 6 {
		t.Fatalf("range [3,6] returned %d records", len(got))
	}
}
