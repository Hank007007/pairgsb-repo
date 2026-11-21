package unredact_test

import (
	"testing"
	"time"

	"maskseal-journal/internal/journal"
	"maskseal-journal/internal/keystore"
	"maskseal-journal/internal/policy"
	"maskseal-journal/internal/record"
	"maskseal-journal/internal/unredact"
)

func setup(t *testing.T) (*journal.Journal, *keystore.Store, record.Record) {
	t.Helper()
	dir := t.TempDir()
	salts, err := keystore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open keystore: %v", err)
	}
	rules, err := policy.Parse([]byte(`{"rules":[{"path":"user.password","action":"redact"}]}`))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	fields, _, err := record.Redact(map[string]any{
		"user": map[string]any{"password": "s3cret", "role": "admin"},
	}, rules, salts)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	j, err := journal.Open(dir)
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}
	t.Cleanup(func() { j.Close() })
	stored, err := j.Append(record.Record{
		Actor:  "alice@example.com",
		Action: "login",
		Time:   time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC),
		Fields: fields,
	})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	return j, salts, stored
}

func TestApproveRequiresSecondPerson(t *testing.T) {
	approver := &unredact.Approver{SigningKey: []byte("key")}
	req := unredact.Request{
		Seq:       1,
		Field:     "user.password",
		Requester: unredact.Identity{ID: "alice", DisplayName: "Alice"},
	}
	tok, err := approver.Approve(req, unredact.Identity{ID: "bob", DisplayName: "Bob"})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if tok == nil || tok.ID == "" {
		t.Fatal("expected a token")
	}
	if err := approver.VerifyToken(tok, req); err != nil {
		t.Fatalf("verify token: %v", err)
	}
}

func TestUnredactRecoversValueAndWritesAudit(t *testing.T) {
	j, salts, stored := setup(t)
	approver := &unredact.Approver{SigningKey: []byte("key")}
	req := unredact.Request{
		Seq:       stored.Seq,
		Field:     "user.password",
		Requester: unredact.Identity{ID: "alice", DisplayName: "Alice"},
	}
	tok, err := approver.Approve(req, unredact.Identity{ID: "bob", DisplayName: "Bob"})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	var delivered string
	result, err := unredact.Unredact(j, salts, approver, unredact.NewRegistry(), req, tok, func(s string) {
		delivered = s
	})
	if err != nil {
		t.Fatalf("unredact: %v", err)
	}
	if delivered != "s3cret" {
		t.Fatalf("delivered %q, want %q", delivered, "s3cret")
	}
	if result.AuditSeq == 0 {
		t.Fatal("expected an audit record")
	}
	audit, err := j.Get(result.AuditSeq)
	if err != nil {
		t.Fatalf("get audit: %v", err)
	}
	if audit.Action != "unredact" {
		t.Fatalf("audit action = %q, want unredact", audit.Action)
	}

	res, err := journal.Verify(j.Dir())
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if res.Records != 2 {
		t.Fatalf("records = %d, want 2", res.Records)
	}
}
