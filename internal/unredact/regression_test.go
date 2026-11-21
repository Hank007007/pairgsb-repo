package unredact_test

import (
	"errors"
	"testing"

	"maskseal-journal/internal/record"
	"maskseal-journal/internal/unredact"
)

func TestApproveUsesIDNotDisplayName(t *testing.T) {
	approver := &unredact.Approver{SigningKey: []byte("key")}
	req := unredact.Request{
		Seq:       1,
		Field:     "user.password",
		Requester: unredact.Identity{ID: "alice@example.com", DisplayName: "Alice"},
	}
	tok, err := approver.Approve(req, unredact.Identity{
		ID:          " Alice@Example.com ",
		DisplayName: "Someone Else",
	})
	if !errors.Is(err, unredact.ErrSelfApproval) {
		t.Fatalf("approve error = %v, want ErrSelfApproval (token %+v)", err, tok)
	}
	if tok != nil {
		t.Fatalf("self approval issued a token: %+v", tok)
	}
}

func TestApproveAllowsDistinctIDsWithSameDisplayName(t *testing.T) {
	approver := &unredact.Approver{SigningKey: []byte("key")}
	req := unredact.Request{
		Seq:       1,
		Field:     "user.password",
		Requester: unredact.Identity{ID: "alice@example.com", DisplayName: "Same Name"},
	}
	tok, err := approver.Approve(req, unredact.Identity{
		ID:          "bob@example.com",
		DisplayName: "Same Name",
	})
	if err != nil {
		t.Fatalf("approve distinct IDs: %v", err)
	}
	if err := approver.VerifyToken(tok, req); err != nil {
		t.Fatalf("verify token: %v", err)
	}
}

func TestTokenIsBoundToSeqAndField(t *testing.T) {
	approver := &unredact.Approver{SigningKey: []byte("key")}
	req := unredact.Request{
		Seq:       7,
		Field:     "user.password",
		Requester: unredact.Identity{ID: "alice"},
	}
	tok, err := approver.Approve(req, unredact.Identity{ID: "bob"})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if err := approver.VerifyToken(tok, req); err != nil {
		t.Fatalf("verify original request: %v", err)
	}

	otherField := req
	otherField.Field = "items[0].ssn"
	if err := approver.VerifyToken(tok, otherField); !errors.Is(err, unredact.ErrBadToken) {
		t.Fatalf("other field error = %v, want ErrBadToken", err)
	}
	otherSeq := req
	otherSeq.Seq = 8
	if err := approver.VerifyToken(tok, otherSeq); !errors.Is(err, unredact.ErrBadToken) {
		t.Fatalf("other seq error = %v, want ErrBadToken", err)
	}

	short := *tok
	short.Signature = append([]byte(nil), tok.Signature[:8]...)
	if err := approver.VerifyToken(&short, req); !errors.Is(err, unredact.ErrBadToken) {
		t.Fatalf("short signature error = %v, want ErrBadToken", err)
	}
}

func TestConsumeRejectsReuse(t *testing.T) {
	registry := unredact.NewRegistry()
	tok := &unredact.Token{ID: "token-1"}
	if err := registry.Consume(tok); err != nil {
		t.Fatalf("first consume: %v", err)
	}
	if err := registry.Consume(tok); !errors.Is(err, unredact.ErrTokenReused) {
		t.Fatalf("second consume error = %v, want ErrTokenReused", err)
	}
}

type failingLog struct {
	rec record.Record
}

func (l *failingLog) Get(seq int64) (record.Record, error) {
	return l.rec, nil
}

func (l *failingLog) Append(record.Record) (record.Record, error) {
	return record.Record{}, errors.New("audit append failed")
}

func TestAuditFailureDisclosesNothing(t *testing.T) {
	_, salts, stored := setup(t)
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

	delivered := ""
	result, err := unredact.Unredact(
		&failingLog{rec: stored},
		salts,
		approver,
		unredact.NewRegistry(),
		req,
		tok,
		func(value string) { delivered = value },
	)
	if err == nil {
		t.Fatal("unredact succeeded even though the audit append failed")
	}
	if result.Plaintext != "" {
		t.Fatalf("plaintext leaked in result: %q", result.Plaintext)
	}
	if result.Salt != nil {
		t.Fatalf("salt leaked in result: %x", result.Salt)
	}
	if result.AuditSeq != 0 {
		t.Fatalf("audit seq = %d, want 0", result.AuditSeq)
	}
	if delivered != "" {
		t.Fatalf("deliver callback received %q", delivered)
	}
}
