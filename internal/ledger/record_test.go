package ledger

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testKey() []byte { return []byte("test-key") }

func TestAppendChainsRecords(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(filepath.Join(dir, "ledger.jsonl"), testKey())
	if err != nil {
		t.Fatal(err)
	}

	r1 := l.NewRecord(KindObservation)
	r1.Fingerprint = Compute(FingerprintInput{RuleID: "r", File: "a.go", Line: 1})
	r1.Status = StatusOpen
	if err := l.Append(r1); err != nil {
		t.Fatal(err)
	}

	r2 := l.NewRecord(KindObservation)
	r2.Fingerprint = Compute(FingerprintInput{RuleID: "r", File: "b.go", Line: 2})
	r2.Status = StatusOpen
	if err := l.Append(r2); err != nil {
		t.Fatal(err)
	}

	records, err := LoadRecords(l.Path())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	if records[0].Seq != 1 || records[1].Seq != 2 {
		t.Fatalf("unexpected sequence numbers: %d, %d", records[0].Seq, records[1].Seq)
	}
	if records[0].Prev != "" {
		t.Fatalf("first record has non-empty prev: %q", records[0].Prev)
	}
	if records[1].Prev != records[0].Hash {
		t.Fatalf("second record does not chain to the first")
	}
	if records[0].Hash != HashRecord(records[0]) {
		t.Fatalf("stored hash does not match recomputed hash")
	}
}

func TestAppendWritesCheckpointOnInterval(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(filepath.Join(dir, "ledger.jsonl"), testKey())
	if err != nil {
		t.Fatal(err)
	}
	l.SetCheckpointEvery(2)

	for i := 0; i < 2; i++ {
		r := l.NewRecord(KindObservation)
		r.Fingerprint = Compute(FingerprintInput{RuleID: "r", File: "a.go", Line: i})
		r.Status = StatusOpen
		if err := l.Append(r); err != nil {
			t.Fatal(err)
		}
	}

	cps, err := LoadCheckpoints(l.Path())
	if err != nil {
		t.Fatal(err)
	}
	if len(cps) != 1 {
		t.Fatalf("got %d checkpoints, want 1", len(cps))
	}
	if cps[0].Seq != 2 {
		t.Fatalf("checkpoint seq = %d, want 2", cps[0].Seq)
	}
	if cps[0].Signature != SignCheckpoint(testKey(), cps[0]) {
		t.Fatalf("checkpoint signature does not verify")
	}
}

func TestOpenMissingLedgerIsEmpty(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(filepath.Join(dir, "missing.jsonl"), testKey())
	if err != nil {
		t.Fatal(err)
	}
	if got := len(l.Records()); got != 0 {
		t.Fatalf("new ledger has %d records, want 0", got)
	}
	if _, err := os.Stat(l.Path()); !os.IsNotExist(err) {
		t.Fatalf("opening a missing ledger should not create it")
	}
}

func TestRecordTimesAreUTC(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(filepath.Join(dir, "ledger.jsonl"), testKey())
	if err != nil {
		t.Fatal(err)
	}
	r := l.NewRecord(KindObservation)
	if r.At.Location() != time.UTC {
		t.Fatalf("record time location = %v, want UTC", r.At.Location())
	}
}

func TestLoadRecordsSkipsBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.jsonl")
	l, err := Open(path, testKey())
	if err != nil {
		t.Fatal(err)
	}
	r := l.NewRecord(KindObservation)
	r.Fingerprint = Compute(FingerprintInput{RuleID: "r", File: "a.go", Line: 1})
	r.Status = StatusOpen
	if err := l.Append(r); err != nil {
		t.Fatal(err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\n\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	records, err := LoadRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
}
