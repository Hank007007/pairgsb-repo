// Package anchor decides when to ask a timestamp authority for a receipt and
// persists the receipts that come back.
package anchor

import "time"

// Policy controls when the chain is anchored.
type Policy struct {
	// Interval is the number of records that should normally make up one
	// anchored interval.
	Interval int
	// MaxAge is the longest the chain may go without a receipt before a new
	// one is requested, regardless of how few records were appended.
	MaxAge time.Duration
}

// State is what the anchoring loop needs to know about the recent past.
type State struct {
	// HeadSeq is the newest record in the local chain.
	HeadSeq int64
	// AnchoredSeq is the last sequence number already covered by a receipt.
	AnchoredSeq int64
	// LastAnchorAt is the wall-clock time of the last successful anchor. The
	// zero value means nothing has been anchored yet.
	LastAnchorAt time.Time
	// Now is the current wall-clock time.
	Now time.Time
}

// Decision explains a ShouldAnchor outcome.
type Decision struct {
	Anchor bool
	// FromSeq and ToSeq describe the interval that should be anchored.
	FromSeq int64
	ToSeq   int64
	Reason  string
}

// ShouldAnchor reports whether a new interval should be anchored and which
// records it should cover.
func (p Policy) ShouldAnchor(st State) Decision {
	if st.HeadSeq <= st.AnchoredSeq {
		return Decision{Reason: "no new records"}
	}
	pending := st.HeadSeq - st.AnchoredSeq
	if p.Interval > 0 && pending >= int64(p.Interval) {
		return Decision{
			Anchor:  true,
			FromSeq: st.AnchoredSeq + 1,
			ToSeq:   st.HeadSeq,
			Reason:  "interval threshold reached",
		}
	}
	if p.MaxAge > 0 && pending >= 1 {
		due := st.LastAnchorAt.IsZero() || !st.Now.Before(st.LastAnchorAt.Add(p.MaxAge))
		if due {
			return Decision{
				Anchor:  true,
				FromSeq: st.AnchoredSeq + 1,
				ToSeq:   st.HeadSeq,
				Reason:  "max age reached",
			}
		}
	}
	return Decision{Reason: "interval threshold not reached"}
}

// DefaultPolicy is a reasonable starting point for short-lived processes.
func DefaultPolicy() Policy {
	return Policy{Interval: 64}
}
