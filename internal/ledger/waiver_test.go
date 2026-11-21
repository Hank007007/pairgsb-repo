package ledger

import (
	"path/filepath"
	"testing"
	"time"
)

func TestWaiverSignAndVerify(t *testing.T) {
	w := NewWaiver("abc123", "false positive, sanitised upstream", "alice", time.Now())
	if err := w.Approve("bob"); err != nil {
		t.Fatal(err)
	}
	w.Sign(testKey())

	if err := w.Verify(testKey()); err != nil {
		t.Fatalf("freshly signed waiver failed verification: %v", err)
	}
}

func TestWaiverVerifyRejectsWrongKey(t *testing.T) {
	w := NewWaiver("abc123", "accepted risk", "alice", time.Now())
	w.Approve("bob")
	w.Sign(testKey())

	if err := w.Verify([]byte("another-key")); err == nil {
		t.Fatal("waiver signed with a different key should not verify")
	}
}

func TestWaiverVerifyRejectsModifiedReason(t *testing.T) {
	w := NewWaiver("abc123", "accepted risk", "alice", time.Now())
	w.Approve("bob")
	w.Sign(testKey())

	tampered := w
	tampered.Reason = "a different reason"
	if err := tampered.Verify(testKey()); err == nil {
		t.Fatal("a waiver with a modified reason should not verify")
	}
}

func TestApproveRequiresAnApprover(t *testing.T) {
	w := NewWaiver("abc123", "accepted risk", "alice", time.Now())
	if err := w.Approve(""); err == nil {
		t.Fatal("approval without a named approver should be rejected")
	}
	if w.ApprovedBy != "" {
		t.Fatalf("approvedBy = %q, want empty", w.ApprovedBy)
	}
}

func TestSubmitWaiverRecordsState(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(filepath.Join(dir, "ledger.jsonl"), testKey())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	fp := Compute(FingerprintInput{RuleID: "r", File: "a.go", Line: 7})

	w, err := l.SubmitWaiver(fp, "accepted risk", "alice", "bob", now.Add(48*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if w.ApprovedBy != "bob" {
		t.Fatalf("approvedBy = %q, want bob", w.ApprovedBy)
	}
	if err := w.Verify(testKey()); err != nil {
		t.Fatalf("recorded waiver does not verify: %v", err)
	}

	state := l.State(fp)
	if !state.Known {
		t.Fatal("waiver did not create a ledger record")
	}
	if state.Status != StatusWaived {
		t.Fatalf("status = %q, want waived", state.Status)
	}
	if got := state.EffectiveStatus(now.Add(time.Hour)); got != StatusWaived {
		t.Fatalf("effective status = %q, want waived", got)
	}
}

func TestLoadKeyUsesEnvironment(t *testing.T) {
	t.Setenv(EnvKey, "from-the-environment")
	key, err := LoadKey()
	if err != nil {
		t.Fatal(err)
	}
	if string(key) != "from-the-environment" {
		t.Fatalf("key = %q, want the environment value", key)
	}
}
