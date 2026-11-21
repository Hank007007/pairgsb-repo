package ledger

import (
	"fmt"
	"time"
)

// Transition applies a status change to a record. The lifecycle itself is
// described in docs/lifecycle.md.
func Transition(r *Record, to Status) error {
	if !to.Valid() {
		return fmt.Errorf("unknown status %q", to)
	}
	if r.Status != "" && !r.Status.Valid() {
		return fmt.Errorf("unknown status %q", r.Status)
	}
	if r.Status == "" {
		r.Status = to
		return nil
	}

	var allowed bool
	switch r.Status {
	case StatusOpen, StatusReopened:
		allowed = to == StatusFixed || to == StatusWaived
	case StatusFixed:
		allowed = to == StatusOpen || to == StatusReopened
	case StatusWaived:
		allowed = to == StatusOpen || to == StatusFixed
	}
	if !allowed {
		return fmt.Errorf("invalid transition %s -> %s", r.Status, to)
	}

	r.Status = to
	return nil
}

// TransitionAt applies a status change together with the time at which it
// takes effect.
func TransitionAt(r *Record, to Status, at time.Time) error {
	if err := Transition(r, to); err != nil {
		return err
	}
	r.At = at.UTC()
	return nil
}

// FindingState is the latest known state of one fingerprint.
type FindingState struct {
	Fingerprint string
	Status      Status
	Waiver      *Waiver
	LastSeq     int64
	Known       bool
}

// State folds every record for a fingerprint into its current state.
func (l *Ledger) State(fingerprint string) FindingState {
	l.mu.RLock()
	defer l.mu.RUnlock()

	state := FindingState{Fingerprint: fingerprint}
	for _, r := range l.records {
		if r.Fingerprint != fingerprint {
			continue
		}
		state.Known = true
		state.LastSeq = r.Seq
		if r.Waiver != nil {
			if err := r.Waiver.Verify(l.key); err != nil {
				// An invalid waiver is treated as though it were never recorded.
				continue
			}
			state.Waiver = r.Waiver
		}
		if r.Status != "" {
			state.Status = r.Status
		}
	}
	return state
}

// EffectiveStatus is the status that should be presented to users and to the
// diff command.
func (s FindingState) EffectiveStatus(now time.Time) Status {
	if s.Status == StatusWaived {
		if s.Waiver != nil && s.Waiver.ActiveAt(now) {
			return StatusWaived
		}
		return StatusOpen
	}
	return s.Status
}
