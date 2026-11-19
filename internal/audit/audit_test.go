package audit

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRecordWritesOneJSONLine(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf)
	err := l.Record(Event{
		Accessor: "svc-payments",
		Path:     "kv/app/payments/stripe",
		Action:   "read",
		Decision: "allow",
		LeaseID:  "lease_000001",
		Backend:  "payments-core",
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	line := strings.TrimSpace(buf.String())
	if strings.Contains(line, "\n") {
		t.Fatal("expected a single line")
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(line), &decoded); err != nil {
		t.Fatalf("decode record: %v", err)
	}
	if decoded["accessor"] != "svc-payments" {
		t.Errorf("accessor = %v", decoded["accessor"])
	}
	if decoded["path"] != "kv/app/payments/stripe" {
		t.Errorf("path = %v", decoded["path"])
	}
	if decoded["action"] != "read" || decoded["decision"] != "allow" {
		t.Errorf("action/decision = %v/%v", decoded["action"], decoded["decision"])
	}
	if decoded["lease_id"] != "lease_000001" {
		t.Errorf("lease_id = %v", decoded["lease_id"])
	}
	if decoded["time"] == nil || decoded["time"] == "" {
		t.Error("expected a timestamp")
	}
}

func TestRecordDenialHasNoLeaseOrBackend(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf)
	if err := l.Record(Event{Accessor: "svc-a", Path: "kv/app/x", Action: "read", Decision: "deny"}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode record: %v", err)
	}
	if _, ok := decoded["lease_id"]; ok {
		t.Error("denial should not carry a lease id")
	}
	if _, ok := decoded["backend"]; ok {
		t.Error("denial should not carry a backend")
	}
}

func TestRecordIsAppendOnly(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf)
	for i := 0; i < 3; i++ {
		if err := l.Record(Event{Accessor: "svc-a", Action: "read", Decision: "allow"}); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
}
