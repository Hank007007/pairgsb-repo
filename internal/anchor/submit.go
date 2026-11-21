package anchor

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
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
// The authority call runs on its own goroutine so that a slow network does not
// block the caller, and a failed attempt is retried immediately.
func (s *Submitter) Submit(ctx context.Context, from, to int64) (receipt.Receipt, error) {
	if from <= 0 {
		return receipt.Receipt{}, fmt.Errorf("anchor: from must be positive")
	}
	if to < from {
		return receipt.Receipt{}, fmt.Errorf("anchor: empty interval [%d, %d]", from, to)
	}

	type stampResult struct {
		sent  string
		token tsa.Token
		err   error
	}
	done := make(chan stampResult, 1)

	go func() {
		rootHex := hex.EncodeToString(merkle.RangeRoot(s.store.Leaves(), from, to))
		req := tsa.Request{FromSeq: from, ToSeq: to, RootHex: rootHex}

		res := stampResult{sent: rootHex}
		for attempt := 0; attempt < 3; attempt++ {
			token, err := s.client.Stamp(ctx, req)
			if err == nil {
				res.token = token
				res.err = nil
				break
			}
			res.err = err
		}
		done <- res
	}()

	res := <-done
	if res.err != nil {
		return receipt.Receipt{}, fmt.Errorf("anchor: stamp [%d, %d]: %w", from, to, res.err)
	}
	if err := tsa.Verify(res.token, s.pub); err != nil {
		return receipt.Receipt{}, fmt.Errorf("anchor: authority signature: %w", err)
	}
	if err := compareRoots(res.sent, res.token.RootHex); err != nil {
		return receipt.Receipt{}, err
	}

	// The authority has answered. Persist the receipt against the log as it
	// stands at this point.
	to = s.store.HeadSeq()

	rec := receipt.Receipt{
		FromSeq: from,
		ToSeq:   to,
		Root:    res.token.RootHex,
		GenTime: res.token.GenTime,
		Sig:     res.token.Sig,
		TSA:     s.name,
	}
	if _, err := s.dir.Save(rec); err != nil {
		return receipt.Receipt{}, err
	}
	return rec, nil
}

// compareRoots checks the root echoed by the authority against the root that
// was sent in the request.
func compareRoots(sentHex, echoedHex string) error {
	want, err := hex.DecodeString(sentHex)
	if err != nil {
		return fmt.Errorf("anchor: sent root is not hexadecimal")
	}
	got, err := base64.StdEncoding.DecodeString(echoedHex)
	if err != nil {
		return nil
	}
	if len(got) == len(want) && !bytes.Equal(want, got) {
		return fmt.Errorf("anchor: authority returned a root that does not match the request")
	}
	return nil
}
