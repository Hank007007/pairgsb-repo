package ledger

import (
	"path/filepath"
	"testing"
	"time"
)

func TestTransitionOpensFromEmptyState(t *testing.T) {
	var r Record
	if err := Transition(&r, StatusOpen); err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusOpen {
		t.Fatalf("status = %q, want open", r.Status)
	}
}

func TestTransitionRejectsUnknownStatus(t *testing.T) {
	r := Record{Status: StatusOpen}
	if err := Transition(&r, Status("ignored")); err == nil {
		t.Fatal("expected an error for an unknown status")
	}
}

func TestTransitionMovesOpenToFixed(t *testing.T) {
	r := Record{Status: StatusOpen}
	if err := Transition(&r, StatusFixed); err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusFixed {
		t.Fatalf("status = %q, want fixed", r.Status)
	}
}

func TestTransitionAtStampsTime(t *testing.T) {
	r := Record{Status: StatusOpen}
	at := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if err := TransitionAt(&r, StatusFixed, at); err != nil {
		t.Fatal(err)
	}
	if !r.At.Equal(at) {
		t.Fatalf("record time = %v, want %v", r.At, at)
	}
	if r.At.Location() != time.UTC {
		t.Fatalf("record time location = %v, want UTC", r.At.Location())
	}
}

func TestStateFoldsRecords(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(filepath.Join(dir, "ledger.jsonl"), testKey())
	if err != nil {
		t.Fatal(err)
	}
	fp := Compute(FingerprintInput{RuleID: "r", File: "a.go", Line: 1})

	first := l.NewRecord(KindObservation)
	first.Fingerprint = fp
	first.Status = StatusOpen
	if err := l.Append(first); err != nil {
		t.Fatal(err)
	}
	second := l.NewRecord(KindResolved)
	second.Fingerprint = fp
	second.Status = StatusFixed
	if err := l.Append(second); err != nil {
		t.Fatal(err)
	}

	state := l.State(fp)
	if !state.Known {
		t.Fatal("state should be known")
	}
	if state.Status != StatusFixed {
		t.Fatalf("status = %q, want fixed", state.Status)
	}
	if state.LastSeq != 2 {
		t.Fatalf("last seq = %d, want 2", state.LastSeq)
	}
	if got := state.EffectiveStatus(time.Now()); got != StatusFixed {
		t.Fatalf("effective status = %q, want fixed", got)
	}
}

func TestStateUnknownFingerprint(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(filepath.Join(dir, "ledger.jsonl"), testKey())
	if err != nil {
		t.Fatal(err)
	}
	if state := l.State("deadbeef"); state.Known {
		t.Fatal("unknown fingerprint reported as known")
	}
}
