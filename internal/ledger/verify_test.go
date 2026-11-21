package ledger

import (
	"path/filepath"
	"testing"
)

func TestVerifyAcceptsFreshLedger(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(filepath.Join(dir, "ledger.jsonl"), testKey())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		r := l.NewRecord(KindObservation)
		r.Fingerprint = Compute(FingerprintInput{RuleID: "r", File: "a.go", Line: i})
		r.Status = StatusOpen
		if err := l.Append(r); err != nil {
			t.Fatal(err)
		}
	}

	report, err := Verify(l.Path(), testKey())
	if err != nil {
		t.Fatalf("Verify failed on a healthy ledger: %v", err)
	}
	if report.CheckedRecords != 3 {
		t.Fatalf("checked %d records, want 3", report.CheckedRecords)
	}
	if !report.OK() {
		t.Fatalf("healthy ledger produced warnings: %v", report.Warnings)
	}
}

func TestVerifyAcceptsLedgerWithCheckpoints(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(filepath.Join(dir, "ledger.jsonl"), testKey())
	if err != nil {
		t.Fatal(err)
	}
	l.SetCheckpointEvery(2)
	for i := 0; i < 4; i++ {
		r := l.NewRecord(KindObservation)
		r.Fingerprint = Compute(FingerprintInput{RuleID: "r", File: "a.go", Line: i})
		r.Status = StatusOpen
		if err := l.Append(r); err != nil {
			t.Fatal(err)
		}
	}

	report, err := Verify(l.Path(), testKey())
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if report.CheckedCheckpoints != 2 {
		t.Fatalf("checked %d checkpoints, want 2", report.CheckedCheckpoints)
	}
}

func TestVerifyEmptyLedger(t *testing.T) {
	dir := t.TempDir()
	report, err := Verify(filepath.Join(dir, "missing.jsonl"), testKey())
	if err != nil {
		t.Fatalf("Verify failed on a missing ledger: %v", err)
	}
	if report.CheckedRecords != 0 {
		t.Fatalf("checked %d records, want 0", report.CheckedRecords)
	}
}
