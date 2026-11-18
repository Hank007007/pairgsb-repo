package recover

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"

	"keycustody/internal/approval"
	"keycustody/internal/shard"
	"keycustody/internal/store"
)

// SensitiveBuffer holds plaintext shard payloads for the duration of a
// recovery attempt. Wipe releases the payloads and zeroes their contents.
type SensitiveBuffer struct {
	payloads [][]byte
}

// Add appends a plaintext payload to the buffer.
func (b *SensitiveBuffer) Add(payload []byte) {
	b.payloads = append(b.payloads, payload)
}

// Len reports how many payloads the buffer currently holds.
func (b *SensitiveBuffer) Len() int {
	return len(b.payloads)
}

// Payloads exposes the buffered plaintext payloads.
func (b *SensitiveBuffer) Payloads() [][]byte {
	return b.payloads
}

// Wipe zeroes every buffered payload and drops the buffer contents.
func (b *SensitiveBuffer) Wipe() {
	for _, payload := range b.payloads {
		for i := range payload {
			payload[i] = 0
		}
	}
	b.payloads = nil
}

// Request describes a single recovery attempt.
type Request struct {
	CustodyID string
	SplitID   string
	Votes     []approval.Vote
	// ShardIDs lists the shards to retrieve, in retrieval order.
	ShardIDs []string
	// Digest is the expected SHA-256 of the recovered master key.
	Digest []byte
}

// Outcome carries the result of a recovery attempt.
type Outcome struct {
	MasterKey []byte
	Shards    int
}

// ErrPolicyDenied is returned when an approver's role is not permitted to
// participate in a recovery.
var ErrPolicyDenied = errors.New("recover: approver role not permitted by policy")

// Flow ties an approval gate, a policy and a shard store together.
type Flow struct {
	Gate   approval.Gate
	Policy approval.Policy
	Store  *store.ShardStore
	Logger *slog.Logger
	// Buffer holds plaintext shards while a recovery is in progress. When
	// nil, Run allocates a buffer for the duration of the attempt.
	Buffer *SensitiveBuffer
}

// Run executes the recovery sequence for req.
func (f *Flow) Run(req Request) (Outcome, error) {
	logger := f.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if f.Buffer == nil {
		f.Buffer = &SensitiveBuffer{}
	}
	buffer := f.Buffer

	gateResult, err := f.Gate.Evaluate(req.Votes)
	if err != nil {
		return Outcome{}, fmt.Errorf("recover: evaluate gate: %w", err)
	}
	if !gateResult.Passed {
		return Outcome{}, fmt.Errorf("recover: approval gate not satisfied: %d approval(s)", gateResult.Approvals)
	}

	for _, vote := range req.Votes {
		if vote.Decision != approval.DecisionApprove {
			continue
		}
		if approval.Match(vote.Role, f.Policy) != approval.EffectAllow {
			return Outcome{}, fmt.Errorf("%w: approver %q role %q", ErrPolicyDenied, vote.ApproverID, vote.Role)
		}
	}

	shards := make([]shard.Shard, 0, len(req.ShardIDs))
	for _, id := range req.ShardIDs {
		plaintext, rec, err := f.Store.Get(req.CustodyID, id)
		if err != nil {
			return Outcome{}, fmt.Errorf("recover: split %q load shard %q: %w", req.SplitID, id, err)
		}
		buffer.Add(plaintext)
		logger.Debug("unsealed shard",
			"split_id", req.SplitID,
			"shard_id", id,
			"custody_id", rec.CustodyID,
			"payload", hex.EncodeToString(plaintext),
		)
		shards = append(shards, shard.Shard{
			ShardID:    rec.ShardID,
			SplitID:    rec.SplitID,
			Generation: rec.Generation,
			Index:      rec.Index,
			Threshold:  rec.Threshold,
			Data:       plaintext,
		})
	}

	masterKey, err := shard.Combine(shards)
	if err != nil {
		return Outcome{}, fmt.Errorf("recover: combine shards: %w", err)
	}

	if len(req.Digest) > 0 {
		sum := sha256.Sum256(masterKey)
		if subtle.ConstantTimeCompare(sum[:], req.Digest) != 1 {
			return Outcome{}, fmt.Errorf("recover: recovered key failed digest verification")
		}
	}

	buffer.Wipe()
	return Outcome{MasterKey: masterKey, Shards: len(shards)}, nil
}
