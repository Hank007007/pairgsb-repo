package store

import (
	"bytes"
	"testing"
)

func newTestStore(t *testing.T) *ShardStore {
	t.Helper()
	s, err := New([]byte("unit-test-store-secret"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestPutGetRoundTrip(t *testing.T) {
	s := newTestStore(t)
	payload := []byte("shard payload bytes")
	if err := s.Put("tenant-alpha", "shard-1", "split-a", 1, 1, 3, payload); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, rec, err := s.Get("tenant-alpha", "shard-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("Get payload = %x, want %x", got, payload)
	}
	if rec.CustodyID != "tenant-alpha" || rec.SplitID != "split-a" || rec.Generation != 1 {
		t.Fatalf("unexpected record metadata: %+v", rec)
	}
}

func TestGetMissingShard(t *testing.T) {
	s := newTestStore(t)
	if _, _, err := s.Get("tenant-alpha", "nope"); err == nil {
		t.Fatal("Get returned no error for a missing shard")
	}
}

func TestGetRejectsEmptyIdentifiers(t *testing.T) {
	s := newTestStore(t)
	if _, _, err := s.Get("", "shard-1"); err == nil {
		t.Fatal("Get accepted an empty custody id")
	}
	if _, _, err := s.Get("tenant-alpha", ""); err == nil {
		t.Fatal("Get accepted an empty shard id")
	}
}

func TestOverwriteKeepsLatestPayload(t *testing.T) {
	s := newTestStore(t)
	if err := s.Put("tenant-alpha", "shard-1", "split-a", 1, 1, 2, []byte("first")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := s.Put("tenant-alpha", "shard-1", "split-a", 2, 1, 2, []byte("second")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, rec, err := s.Get("tenant-alpha", "shard-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "second" {
		t.Fatalf("Get payload = %q, want %q", got, "second")
	}
	if rec.Generation != 2 {
		t.Fatalf("generation = %d, want 2", rec.Generation)
	}
}

func TestListReturnsOwnedRecords(t *testing.T) {
	s := newTestStore(t)
	if err := s.Put("tenant-alpha", "shard-1", "split-a", 1, 1, 2, []byte("a")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := s.Put("tenant-beta", "shard-2", "split-b", 1, 1, 2, []byte("b")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	records := s.List("tenant-alpha")
	if len(records) != 1 || records[0].ShardID != "shard-1" {
		t.Fatalf("List(tenant-alpha) = %+v", records)
	}
	if s.Len() != 2 {
		t.Fatalf("Len = %d, want 2", s.Len())
	}
}

func TestTamperedCiphertextFailsToOpen(t *testing.T) {
	s := newTestStore(t)
	if err := s.Put("tenant-alpha", "shard-1", "split-a", 1, 1, 2, []byte("payload")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	s.mu.Lock()
	rec := s.records["shard-1"]
	rec.Ciphertext[len(rec.Ciphertext)-1] ^= 0xff
	s.records["shard-1"] = rec
	s.mu.Unlock()

	if _, _, err := s.Get("tenant-alpha", "shard-1"); err == nil {
		t.Fatal("Get opened tampered ciphertext")
	}
}
