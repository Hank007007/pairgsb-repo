package journal_test

import (
	"testing"
	"time"

	"maskseal-journal/internal/journal"
	"maskseal-journal/internal/record"
)

func testRecord(action string) record.Record {
	return record.Record{
		Actor:  "alice@example.com",
		Action: action,
		Time:   time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC),
		Fields: map[string]any{"k": "v"},
	}
}

func TestAppendChainsRecords(t *testing.T) {
	dir := t.TempDir()
	j, err := journal.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	first, err := j.Append(testRecord("login"))
	if err != nil {
		t.Fatalf("append first: %v", err)
	}
	second, err := j.Append(testRecord("logout"))
	if err != nil {
		t.Fatalf("append second: %v", err)
	}
	if err := j.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if first.Seq != 1 || second.Seq != 2 {
		t.Fatalf("seqs = %d, %d; want 1, 2", first.Seq, second.Seq)
	}
	if first.PrevHash != record.GenesisHash {
		t.Fatalf("first prev_hash = %s, want genesis", first.PrevHash)
	}
	if second.PrevHash != first.Hash {
		t.Fatalf("second prev_hash = %s, want %s", second.PrevHash, first.Hash)
	}

	result, err := journal.Verify(dir)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Records != 2 || result.LastSeq != 2 {
		t.Fatalf("verify result = %+v, want 2 records ending at seq 2", result)
	}
}

func TestOpenResumesChain(t *testing.T) {
	dir := t.TempDir()
	j, err := journal.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	first, err := j.Append(testRecord("login"))
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := j.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	reopened, err := journal.Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()
	second, err := reopened.Append(testRecord("logout"))
	if err != nil {
		t.Fatalf("append after reopen: %v", err)
	}
	if second.Seq != 2 {
		t.Fatalf("seq after reopen = %d, want 2", second.Seq)
	}
	if second.PrevHash != first.Hash {
		t.Fatalf("prev_hash after reopen = %s, want %s", second.PrevHash, first.Hash)
	}
}

func TestGetReturnsStoredRecord(t *testing.T) {
	dir := t.TempDir()
	j, err := journal.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer j.Close()
	stored, err := j.Append(testRecord("login"))
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	got, err := j.Get(stored.Seq)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Hash != stored.Hash || got.Action != "login" {
		t.Fatalf("got %+v, want %+v", got, stored)
	}
}
