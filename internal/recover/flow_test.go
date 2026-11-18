package recover

import (
	"bytes"
	"crypto/sha256"
	"log/slog"
	"testing"

	"keycustody/internal/approval"
	"keycustody/internal/shard"
	"keycustody/internal/store"
)

const testSecret = "the threshold master key for test use"

func newFixture(t *testing.T) (*Flow, []shard.Shard, []byte) {
	t.Helper()

	shards, err := shard.Split([]byte(testSecret), 5, 3, "split-test", 1)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}

	shardStore, err := store.New([]byte("recover-test-store-secret"))
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	for _, s := range shards {
		if err := shardStore.Put("tenant-alpha", s.ShardID, s.SplitID, s.Generation, s.Index, s.Threshold, s.Data); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}

	flow := &Flow{
		Gate:   approval.Gate{Required: 2},
		Policy: approval.Policy{{Role: "ops", Effect: approval.EffectAllow}},
		Store:  shardStore,
		Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	}
	digest := sha256.Sum256([]byte(testSecret))
	return flow, shards, digest[:]
}

func approve(names ...string) []approval.Vote {
	votes := make([]approval.Vote, 0, len(names))
	for _, n := range names {
		votes = append(votes, approval.Vote{
			ApproverID: n,
			Role:       "ops-oncall",
			Decision:   approval.DecisionApprove,
		})
	}
	return votes
}

func TestRunRecoversMasterKey(t *testing.T) {
	flow, shards, digest := newFixture(t)

	ids := []string{shards[0].ShardID, shards[2].ShardID, shards[4].ShardID}
	outcome, err := flow.Run(Request{
		CustodyID: "tenant-alpha",
		SplitID:   "split-test",
		Votes:     approve("alice", "bob"),
		ShardIDs:  ids,
		Digest:    digest,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if string(outcome.MasterKey) != testSecret {
		t.Fatalf("recovered key = %q", outcome.MasterKey)
	}
	if outcome.Shards != len(ids) {
		t.Fatalf("Shards = %d, want %d", outcome.Shards, len(ids))
	}
	if flow.Buffer.Len() != 0 {
		t.Fatalf("buffer retained %d payloads after a successful run", flow.Buffer.Len())
	}
}

func TestRunRequiresApprovalGate(t *testing.T) {
	flow, shards, digest := newFixture(t)

	_, err := flow.Run(Request{
		CustodyID: "tenant-alpha",
		SplitID:   "split-test",
		Votes:     approve("alice"),
		ShardIDs:  []string{shards[0].ShardID, shards[1].ShardID, shards[2].ShardID},
		Digest:    digest,
	})
	if err == nil {
		t.Fatal("Run recovered a key without enough approvals")
	}
}

func TestRunRejectsDeniedRole(t *testing.T) {
	flow, shards, digest := newFixture(t)
	flow.Policy = approval.Policy{{Role: "auditor", Effect: approval.EffectDeny}}

	votes := []approval.Vote{
		{ApproverID: "alice", Role: "auditor", Decision: approval.DecisionApprove},
		{ApproverID: "bob", Role: "ops-oncall", Decision: approval.DecisionApprove},
	}
	_, err := flow.Run(Request{
		CustodyID: "tenant-alpha",
		SplitID:   "split-test",
		Votes:     votes,
		ShardIDs:  []string{shards[0].ShardID, shards[1].ShardID, shards[2].ShardID},
		Digest:    digest,
	})
	if err == nil {
		t.Fatal("Run allowed an approver whose role is denied")
	}
}

func TestRunDetectsDigestMismatch(t *testing.T) {
	flow, shards, _ := newFixture(t)

	_, err := flow.Run(Request{
		CustodyID: "tenant-alpha",
		SplitID:   "split-test",
		Votes:     approve("alice", "bob"),
		ShardIDs:  []string{shards[0].ShardID, shards[1].ShardID, shards[2].ShardID},
		Digest:    digestOf("a different key"),
	})
	if err == nil {
		t.Fatal("Run accepted a key that does not match the digest")
	}
}

func TestRunReportsMissingShard(t *testing.T) {
	flow, shards, digest := newFixture(t)

	_, err := flow.Run(Request{
		CustodyID: "tenant-alpha",
		SplitID:   "split-test",
		Votes:     approve("alice", "bob"),
		ShardIDs:  []string{shards[0].ShardID, shards[1].ShardID, "does-not-exist"},
		Digest:    digest,
	})
	if err == nil {
		t.Fatal("Run succeeded with a missing shard")
	}
}

func TestSensitiveBufferWipe(t *testing.T) {
	buf := &SensitiveBuffer{}
	payload := []byte("transient plaintext")
	buf.Add(payload)
	if buf.Len() != 1 {
		t.Fatalf("Len = %d, want 1", buf.Len())
	}
	buf.Wipe()
	if buf.Len() != 0 {
		t.Fatalf("Len after Wipe = %d", buf.Len())
	}
	for i, b := range payload {
		if b != 0 {
			t.Fatalf("payload byte %d not zeroed", i)
		}
	}
}

func digestOf(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}
