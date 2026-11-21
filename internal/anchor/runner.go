// Package anchor decides when to ask a timestamp authority for a receipt and
// persists the receipts that come back.
package anchor

import (
	"context"
	"sync"
	"time"

	"beaconproof/internal/receipt"
)

// Runner drives anchoring: it asks the policy whether an interval is due and
// submits it.
type Runner struct {
	policy    Policy
	submitter *Submitter
	dir       *receipt.Dir
	now       func() time.Time

	mu           sync.Mutex
	anchoredSeq  int64
	lastAnchorAt time.Time
}

// NewRunner builds a runner. It initialises the anchored position from the
// receipts already on disk.
func NewRunner(policy Policy, submitter *Submitter, dir *receipt.Dir) (*Runner, error) {
	r := &Runner{
		policy:    policy,
		submitter: submitter,
		dir:       dir,
		now:       time.Now,
	}
	stored, err := dir.LoadAll()
	if err != nil {
		return nil, err
	}
	for _, rec := range stored {
		if rec.ToSeq > r.anchoredSeq {
			r.anchoredSeq = rec.ToSeq
		}
		if gen, err := time.Parse(time.RFC3339, rec.GenTime); err == nil {
			if gen.After(r.lastAnchorAt) {
				r.lastAnchorAt = gen
			}
		}
	}
	return r, nil
}

// WithNow replaces the clock used for policy decisions. It is intended for
// tests.
func (r *Runner) WithNow(now func() time.Time) *Runner {
	if now != nil {
		r.now = now
	}
	return r
}

// AnchoredSeq reports the last sequence number covered by a stored receipt.
func (r *Runner) AnchoredSeq() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.anchoredSeq
}

// Tick performs one policy evaluation. When the policy asks for an anchor the
// runner submits the policy's pending range as consecutive, non-overlapping
// chunks of at most Interval records.
func (r *Runner) Tick(ctx context.Context, headSeq int64) (Decision, error) {
	r.mu.Lock()
	st := State{
		HeadSeq:      headSeq,
		AnchoredSeq:  r.anchoredSeq,
		LastAnchorAt: r.lastAnchorAt,
		Now:          r.now(),
	}
	dec := r.policy.ShouldAnchor(st)
	r.mu.Unlock()

	if !dec.Anchor {
		return dec, nil
	}

	for from := dec.FromSeq; from <= dec.ToSeq; {
		to := dec.ToSeq
		if r.policy.Interval > 0 {
			limit := from + int64(r.policy.Interval) - 1
			if limit < to {
				to = limit
			}
		}

		rec, err := r.submitter.Submit(ctx, from, to)
		if err != nil {
			// The failed chunk stays unanchored; the next tick retries it.
			return dec, err
		}

		r.mu.Lock()
		if rec.ToSeq > r.anchoredSeq {
			r.anchoredSeq = rec.ToSeq
		}
		if gen, err := time.Parse(time.RFC3339, rec.GenTime); err == nil {
			r.lastAnchorAt = gen
		} else {
			r.lastAnchorAt = r.now()
		}
		r.mu.Unlock()

		from = rec.ToSeq + 1
	}
	return dec, nil
}
