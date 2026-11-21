package record_test

import (
	"testing"
	"time"

	"maskseal-journal/internal/keystore"
	"maskseal-journal/internal/policy"
	"maskseal-journal/internal/record"
)

func mustPolicy(t *testing.T, doc string) *policy.RuleSet {
	t.Helper()
	rs, err := policy.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	return rs
}

func TestRedactReplacesMatchedScalarWithCommitment(t *testing.T) {
	salts, err := keystore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open keystore: %v", err)
	}
	rules := mustPolicy(t, `{"rules":[{"path":"password","action":"redact"}]}`)

	fields := map[string]any{
		"password": "hunter2",
		"action":   "login",
	}
	out, sealed, err := record.Redact(fields, rules, salts)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}

	value, ok := out["password"]
	if !ok {
		t.Fatal("password missing from redacted fields")
	}
	red, ok := value.(*record.Redacted)
	if !ok {
		t.Fatalf("password was not replaced by a commitment: %T", value)
	}
	if red.Commitment == "" {
		t.Fatal("commitment is empty")
	}
	if red.SaltID == "" {
		t.Fatal("salt id is empty")
	}
	if len(sealed) != 1 || sealed[0].Path != "password" {
		t.Fatalf("unexpected sealed values: %+v", sealed)
	}

	entry, err := salts.Get(keystore.SaltID(red.SaltID))
	if err != nil {
		t.Fatalf("salt entry: %v", err)
	}
	if len(entry.Salt) == 0 {
		t.Fatal("stored salt is empty")
	}
	want, err := record.Commit(entry.Salt, "password", "hunter2")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if want != red.Commitment {
		t.Fatalf("commitment = %s, want %s", red.Commitment, want)
	}
}

func TestRedactLeavesKeptFieldAlone(t *testing.T) {
	salts, err := keystore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open keystore: %v", err)
	}
	rules := mustPolicy(t, `{"rules":[
		{"path":"password","action":"redact"},
		{"path":"role","action":"keep"}
	]}`)

	out, _, err := record.Redact(map[string]any{
		"password": "hunter2",
		"role":     "admin",
	}, rules, salts)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	if got, ok := out["role"].(string); !ok || got != "admin" {
		t.Fatalf("role = %#v, want admin", out["role"])
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	now := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	rec := record.Record{
		Seq:      1,
		Actor:    "alice@example.com",
		Action:   "login",
		Time:     now,
		PrevHash: record.GenesisHash,
		Fields: map[string]any{
			"role": "admin",
			"note": "ok",
		},
	}
	rec.Hash = record.Hash(rec)

	line, err := record.Encode(rec)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := record.Decode(line)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Seq != rec.Seq || got.Actor != rec.Actor || got.Action != rec.Action {
		t.Fatalf("decoded header mismatch: %+v", got)
	}
	if !got.Time.Equal(now) {
		t.Fatalf("decoded time = %v, want %v", got.Time, now)
	}
	if got.Hash != rec.Hash {
		t.Fatalf("decoded hash = %s, want %s", got.Hash, rec.Hash)
	}
	if got.Fields["role"] != "admin" || got.Fields["note"] != "ok" {
		t.Fatalf("decoded fields mismatch: %#v", got.Fields)
	}
}

func TestHashChangesWithActor(t *testing.T) {
	base := record.Record{Seq: 1, Actor: "alice", Action: "login", PrevHash: record.GenesisHash}
	other := base
	other.Actor = "bob"
	if record.Hash(base) == record.Hash(other) {
		t.Fatal("hash did not change when actor changed")
	}
}
