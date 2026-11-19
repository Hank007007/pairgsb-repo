package recover

import (
	"bytes"
	"encoding/hex"
	"log/slog"
	"strings"
	"testing"

	"keycustody/internal/approval"
)

// TestRunWipesBufferAfterMissingShard reproduces the review finding: a
// failed retrieval must still zero and release every buffered plaintext.
func TestRunWipesBufferAfterMissingShard(t *testing.T) {
	flow, shards, digest := newFixture(t)

	_, err := flow.Run(Request{
		CustodyID: "tenant-alpha",
		SplitID:   "split-test",
		Votes:     approve("alice", "bob"),
		ShardIDs:  []string{shards[0].ShardID, shards[1].ShardID, "split-f-9"},
		Digest:    digest,
	})
	if err == nil {
		t.Fatal("Run succeeded with a non-existent shard id")
	}
	if flow.Buffer.Len() != 0 {
		t.Fatalf("buffer retained %d plaintext payloads after a failed run", flow.Buffer.Len())
	}
	if flow.Buffer.Payloads() != nil {
		t.Fatal("buffer still references payload slices after a failed run")
	}
}

// TestRunWipesBufferAfterDigestMismatch ensures a digest failure on the
// fully buffered shards also triggers cleanup.
func TestRunWipesBufferAfterDigestMismatch(t *testing.T) {
	flow, shards, _ := newFixture(t)

	_, err := flow.Run(Request{
		CustodyID: "tenant-alpha",
		SplitID:   "split-test",
		Votes:     approve("alice", "bob"),
		ShardIDs:  []string{shards[0].ShardID, shards[1].ShardID, shards[2].ShardID},
		Digest:    digestOf("the wrong expected digest"),
	})
	if err == nil {
		t.Fatal("Run accepted a key that fails digest verification")
	}
	if flow.Buffer.Len() != 0 {
		t.Fatalf("buffer retained %d plaintext payloads after a digest failure", flow.Buffer.Len())
	}
}

// TestRunWipesBufferAfterGateFailure covers the early-exit path.
func TestRunWipesBufferAfterGateFailure(t *testing.T) {
	flow, shards, digest := newFixture(t)

	_, err := flow.Run(Request{
		CustodyID: "tenant-alpha",
		SplitID:   "split-test",
		Votes:     approve("alice"),
		ShardIDs:  []string{shards[0].ShardID, shards[1].ShardID, shards[2].ShardID},
		Digest:    digest,
	})
	if err == nil {
		t.Fatal("Run recovered a key without passing the gate")
	}
	if flow.Buffer.Len() != 0 {
		t.Fatalf("buffer retained %d payloads after a gate failure", flow.Buffer.Len())
	}
}

// TestRunNeverLogsShardBytes reproduces the §8.5 finding: even at debug
// level, shard payload bytes (or encodings of them) must never be logged.
func TestRunNeverLogsShardBytes(t *testing.T) {
	flow, shards, digest := newFixture(t)
	var logBuf bytes.Buffer
	flow.Logger = slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ids := []string{shards[0].ShardID, shards[2].ShardID, shards[4].ShardID}
	if _, err := flow.Run(Request{
		CustodyID: "tenant-alpha",
		SplitID:   "split-test",
		Votes:     approve("alice", "bob"),
		ShardIDs:  ids,
		Digest:    digest,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	logged := logBuf.String()
	if !strings.Contains(logged, "unsealed shard") {
		t.Fatal("expected metadata-level unsealed shard log line")
	}
	if strings.Contains(strings.ToLower(logged), "payload") {
		t.Fatalf("log output contains a payload field: %s", logged)
	}
	for _, s := range shards {
		encoded := hex.EncodeToString(s.Data)
		if strings.Contains(logged, encoded) {
			t.Fatalf("log output contains shard bytes for %s", s.ShardID)
		}
	}
}

// TestRunBufferCanBeReusedAfterFailure ensures a wiped buffer still serves a
// later successful attempt and ends empty again.
func TestRunBufferCanBeReusedAfterFailure(t *testing.T) {
	flow, shards, digest := newFixture(t)

	_, _ = flow.Run(Request{
		CustodyID: "tenant-alpha",
		SplitID:   "split-test",
		Votes:     approve("alice", "bob"),
		ShardIDs:  []string{shards[0].ShardID, "missing-id"},
		Digest:    digest,
	})

	outcome, err := flow.Run(Request{
		CustodyID: "tenant-alpha",
		SplitID:   "split-test",
		Votes:     approve("alice", "bob"),
		ShardIDs:  []string{shards[0].ShardID, shards[2].ShardID, shards[4].ShardID},
		Digest:    digest,
	})
	if err != nil {
		t.Fatalf("retry Run: %v", err)
	}
	if string(outcome.MasterKey) != testSecret {
		t.Fatalf("retry recovered key = %q", outcome.MasterKey)
	}
	if flow.Buffer.Len() != 0 {
		t.Fatalf("buffer retained %d payloads after a successful retry", flow.Buffer.Len())
	}
}

func TestRunDeniedRoleStillCleansBuffer(t *testing.T) {
	flow, shards, digest := newFixture(t)
	flow.Policy = approval.Policy{{Role: "auditor", Effect: approval.EffectDeny}}

	_, err := flow.Run(Request{
		CustodyID: "tenant-alpha",
		SplitID:   "split-test",
		Votes: []approval.Vote{
			{ApproverID: "alice", Role: "auditor", Decision: approval.DecisionApprove},
			{ApproverID: "bob", Role: "auditor", Decision: approval.DecisionApprove},
		},
		ShardIDs: []string{shards[0].ShardID, shards[1].ShardID, shards[2].ShardID},
		Digest:   digest,
	})
	if err == nil {
		t.Fatal("Run allowed a denied role")
	}
	if flow.Buffer.Len() != 0 {
		t.Fatalf("buffer retained %d payloads after a policy denial", flow.Buffer.Len())
	}
}
