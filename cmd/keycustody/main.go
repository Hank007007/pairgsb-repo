// Command keycustody demonstrates a threshold key custody round trip:
// split a master key into shards, place the encrypted shards in a custody
// store, collect approvals and recover the key.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"keycustody/internal/approval"
	"keycustody/internal/recover"
	"keycustody/internal/shard"
	"keycustody/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "keycustody: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	mode := flag.String("mode", "demo", "operation to perform: demo")
	secret := flag.String("secret", "correct horse battery staple", "master key material")
	total := flag.Int("total", 5, "number of shards to create")
	threshold := flag.Int("threshold", 3, "number of shards required to recover")
	splitID := flag.String("split-id", "split-a", "identifier of the splitting operation")
	generation := flag.Int("generation", 1, "generation of the splitting operation")
	custodyID := flag.String("custody-id", "tenant-alpha", "custody domain owning the shards")
	approvers := flag.String("approvers", "alice,bob,carol", "comma separated approver ids")
	required := flag.Int("required", 2, "required number of distinct approvers")
	role := flag.String("role", "ops-oncall", "role assigned to demo approvers")
	verbose := flag.Bool("verbose", false, "emit debug logging")
	flag.Parse()

	if *mode != "demo" {
		return fmt.Errorf("unsupported mode %q", *mode)
	}

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	shards, err := shard.Split([]byte(*secret), *total, *threshold, *splitID, *generation)
	if err != nil {
		return fmt.Errorf("split: %w", err)
	}

	storeSecret := []byte("demo-store-secret-change-me")
	shardStore, err := store.New(storeSecret)
	if err != nil {
		return fmt.Errorf("create store: %w", err)
	}
	for _, s := range shards {
		if err := shardStore.Put(*custodyID, s.ShardID, s.SplitID, s.Generation, s.Index, s.Threshold, s.Data); err != nil {
			return fmt.Errorf("store shard: %w", err)
		}
	}
	logger.Info("shards stored", "count", len(shards), "custody_id", *custodyID, "split_id", *splitID, "generation", *generation)

	ids := make([]string, 0, *threshold)
	for _, s := range shards[:*threshold] {
		ids = append(ids, s.ShardID)
	}

	var votes []approval.Vote
	for _, name := range strings.Split(*approvers, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		votes = append(votes, approval.Vote{
			ApproverID: name,
			Role:       *role,
			Decision:   approval.DecisionApprove,
		})
	}

	policy := approval.Policy{
		{Role: "admin", Effect: approval.EffectAllow},
		{Role: "ops-oncall", Effect: approval.EffectAllow},
	}
	gate := approval.Gate{Required: *required}

	flow := &recover.Flow{Gate: gate, Policy: policy, Store: shardStore, Logger: logger}
	sum := sha256.Sum256([]byte(*secret))
	outcome, err := flow.Run(recover.Request{
		CustodyID: *custodyID,
		SplitID:   *splitID,
		Votes:     votes,
		ShardIDs:  ids,
		Digest:    sum[:],
	})
	if err != nil {
		return fmt.Errorf("recover: %w", err)
	}

	recovered := sha256.Sum256(outcome.MasterKey)
	fmt.Printf("recovered master key (%d bytes) from %d shards\n", len(outcome.MasterKey), outcome.Shards)
	fmt.Printf("sha256: %s\n", hex.EncodeToString(recovered[:]))
	fmt.Println("digest verified")
	return nil
}
