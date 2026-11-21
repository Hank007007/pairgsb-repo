// Package chain implements the local append-only audit log.
//
// Each record commits to the hash of the record before it, so the log is a
// hash chain. Records are persisted as JSON lines: one record per line, in
// append order.
package chain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// Record is a single audit event together with its position in the chain.
type Record struct {
	Seq    int64  `json:"seq"`
	Time   string `json:"ts"`
	Actor  string `json:"actor"`
	Action string `json:"action"`
	Body   string `json:"body"`
	Prev   string `json:"prev"`
	Hash   string `json:"hash"`
}

// Canonical serialises the fields that are covered by the record hash.
//
// The layout is described in docs/chain-format.md. Fields are joined with the
// unit separator byte.
func (r Record) Canonical() string {
	return strings.Join([]string{
		r.Time,
		r.Actor,
		r.Action,
		r.Body,
	}, "\x1f")
}

// ComputeHash derives the chain hash for a record from its contents.
func ComputeHash(r Record) string {
	sum := sha256.Sum256([]byte(r.Canonical()))
	return hex.EncodeToString(sum[:])
}

// HashRecord is the functional form used while building a new record.
func HashRecord(seq int64, ts, actor, action, body, prev string) string {
	return ComputeHash(Record{
		Seq:    seq,
		Time:   ts,
		Actor:  actor,
		Action: action,
		Body:   body,
		Prev:   prev,
	})
}

// ParseRecord decodes one JSON line into a Record.
func ParseRecord(line []byte) (Record, error) {
	var r Record
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return Record{}, fmt.Errorf("decode record: %w", err)
	}
	return r, nil
}
