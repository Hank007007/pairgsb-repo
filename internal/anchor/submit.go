package anchor

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"time"

	"beaconproof/internal/chain"
	"beaconproof/internal/merkle"
	"beaconproof/internal/receipt"
	"beaconproof/internal/tsa"
)

// Submitter turns one chain interval into one stored receipt.
type Submitter struct {
	store  *chain.Store
	dir    *receipt.Dir
	client *tsa.Client
	pub    ed25519.PublicKey
	name   string
	now    func() time.Time
}

// NewSubmitter wires a submitter to a log, a receipt directory and an
// authority.
func NewSubmitter(store *chain.Store, dir *receipt.Dir, client *tsa.Client, pub ed25519.PublicKey, name string) *Submitter {
	return &Submitter{
		store:  store,
		dir:    dir,
		client: client,
		pub:    pub,
		name:   name,
		now:    time.Now,
	}
}

// WithNow replaces the clock used for bookkeeping. It is intended for tests.
func (s *Submitter) WithNow(now func() time.Time) *Submitter {
	if now != nil {
		s.now = now
	}
	return s
}

// Submit asks the authority to stamp [from, to] and stores the receipt it
// returns.
//
// The request carries the interval's Merkle root as lower-case hexadecimal.
// The interval and its root are snapshotted before the request leaves; a
// failed attempt returns an error and the caller retries the same interval.
func (s *Submitter) Submit(ctx context.Context, from, to int64) (receipt.Receipt, error) {
	if from <= 0 {
		return receipt.Receipt{}, fmt.Errorf("anchor: from must be positive")
	}
	if to < from {
		return receipt.Receipt{}, fmt.Errorf("anchor: empty interval [%d, %d]", from, to)
	}

	// Take one snapshot of the interval before the request leaves. Records
	// appended while the request is in flight belong to a later interval.
	root, err := s.intervalRoot(from, to)
	if err != nil {
		return receipt.Receipt{}, err
	}
	sentRoot := hex.EncodeToString(root)
	req := tsa.Request{FromSeq: from, ToSeq: to, RootHex: sentRoot}

	token, err := s.client.Stamp(ctx, req)
	if err != nil {
		return receipt.Receipt{}, fmt.Errorf("anchor: stamp [%d, %d]: %w", from, to, err)
	}
	if err := tsa.Verify(token, s.pub); err != nil {
		return receipt.Receipt{}, fmt.Errorf("anchor: authority signature: %w", err)
	}
	if err := compareRoots(sentRoot, token.RootHex); err != nil {
		return receipt.Receipt{}, err
	}
	if _, err := time.Parse(time.RFC3339, token.GenTime); err != nil {
		return receipt.Receipt{}, fmt.Errorf("anchor: authority returned an invalid generation time: %w", err)
	}

	rec := receipt.Receipt{
		FromSeq: from,
		ToSeq:   to,
		Root:    token.RootHex,
		GenTime: token.GenTime,
		Sig:     token.Sig,
		TSA:     s.name,
	}
	if _, err := s.dir.Save(rec); err != nil {
		return receipt.Receipt{}, err
	}
	return rec, nil
}

// intervalRoot computes the root over exactly [from, to] from a snapshot of
// the local records.
func (s *Submitter) intervalRoot(from, to int64) ([]byte, error) {
	records := s.store.Records()
	bySeq := make(map[int64]chain.Record, len(records))
	for _, rec := range records {
		bySeq[rec.Seq] = rec
	}

	count := to - from + 1
	if count <= 0 || count > int64(len(bySeq)) {
		return nil, fmt.Errorf("anchor: interval [%d, %d] is not fully covered by local records", from, to)
	}
	leaves := make(map[int64][]byte, count)
	for seq := from; seq <= to; seq++ {
		rec, ok := bySeq[seq]
		if !ok {
			return nil, fmt.Errorf("anchor: interval [%d, %d] is not fully covered by local records", from, to)
		}
		raw, err := hex.DecodeString(rec.Hash)
		if err != nil {
			return nil, fmt.Errorf("anchor: record %d has an invalid hash: %w", seq, err)
		}
		leaves[seq] = raw
	}
	root := merkle.RangeRoot(leaves, from, to)
	if root == nil {
		return nil, fmt.Errorf("anchor: no Merkle root for [%d, %d]", from, to)
	}
	return root, nil
}

// compareRoots checks the root echoed by the authority against the root that
// was sent in the request.
func compareRoots(sentHex, echoedHex string) error {
	want, err := hex.DecodeString(sentHex)
	if err != nil {
		return fmt.Errorf("anchor: sent root is not hexadecimal")
	}
	got, err := hex.DecodeString(echoedHex)
	if err != nil {
		return fmt.Errorf("anchor: authority returned a root that is not hexadecimal")
	}
	if !bytes.Equal(want, got) {
		return fmt.Errorf("anchor: authority returned a root that does not match the request")
	}
	return nil
}
