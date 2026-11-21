package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// ZeroHash is the prev value of the first record in a store.
const ZeroHash = "0000000000000000000000000000000000000000000000000000000000000000"

const recordTag = "witnesschain/v1/record"

// Record is one audit event.
//
// Serialization order follows docs/chain-format.md §2.1.
type Record struct {
	Seq     uint64 `json:"seq"`
	TS      string `json:"ts"`
	Actor   string `json:"actor"`
	Action  string `json:"action"`
	Payload string `json:"payload"`
	Outcome string `json:"outcome"`
	Prev    string `json:"prev"`
	Hash    string `json:"hash"`
}

// NewRecord builds a record with a canonical timestamp and computed hash.
func NewRecord(seq uint64, t time.Time, actor, action, payload, outcome, prev string) *Record {
	r := &Record{
		Seq:     seq,
		TS:      t.UTC().Format(time.RFC3339),
		Actor:   actor,
		Action:  action,
		Payload: payload,
		Outcome: outcome,
		Prev:    prev,
	}
	r.Hash = r.ComputeHash()
	return r
}

// ComputeHash returns the record hash defined in docs/chain-format.md §3.
func (r *Record) ComputeHash() string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\n", recordTag)
	fmt.Fprintf(h, "%d\n", r.Seq)
	fmt.Fprintf(h, "%s\n", r.TS)
	fmt.Fprintf(h, "%s\n", r.Actor)
	fmt.Fprintf(h, "%s\n", r.Action)
	return hex.EncodeToString(h.Sum(nil))
}

// EncodeLine serializes the record as one compact JSON line with trailing LF.
func (r *Record) EncodeLine() ([]byte, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// ParseRecord parses one JSON line (without the LF) into a Record.
func ParseRecord(line []byte) (*Record, error) {
	var r Record
	if err := json.Unmarshal(line, &r); err != nil {
		return nil, fmt.Errorf("parse record: %w", err)
	}
	return &r, nil
}

// Validate checks the record's stored hash against a recomputation.
func (r *Record) Validate() error {
	if got := r.ComputeHash(); got != r.Hash {
		return fmt.Errorf("hash mismatch: stored %s, computed %s", r.Hash, got)
	}
	return nil
}

// String renders the record for diagnostics.
func (r *Record) String() string {
	return fmt.Sprintf("seq=%d ts=%s actor=%s action=%s outcome=%s", r.Seq, r.TS, r.Actor, r.Action, r.Outcome)
}
