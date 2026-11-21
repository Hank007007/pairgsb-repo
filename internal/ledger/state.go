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
	if r.Status == "" {
		r.Status = to
		return nil
	}
	if r.Status == StatusFixed && to == StatusOpen {
		r.Status = to
		return nil
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
	state := FindingState{Fingerprint: fingerprint}
	for _, r := range l.records {
		if r.Fingerprint != fingerprint {
			continue
		}
		state.Known = true
		state.LastSeq = r.Seq
		if r.Status != "" {
			state.Status = r.Status
		}
		if r.Waiver != nil {
			state.Waiver = r.Waiver
		}
	}
	return state
}

// EffectiveStatus is the status that should be presented to users and to the
// diff command.
func (s FindingState) EffectiveStatus(now time.Time) Status {
	if s.Status == StatusWaived && s.Waiver != nil {
		return StatusWaived
	}
	return s.Status
}
