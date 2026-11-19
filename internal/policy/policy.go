// Package policy describes the per-key limits that gate signing requests.
//
// The normative rules live in docs/agent-policy.md. This package only
// exposes the data and the small amount of bookkeeping that the agent needs
// to apply them.
package policy

import (
	"encoding/json"
	"time"
)

// Policy is the set of limits attached to a single key.
type Policy struct {
	// Purposes is the allow-list of purposes this key may be used for.
	Purposes []string `json:"purposes"`

	// MaxSignaturesPerHour caps the number of signatures issued inside one
	// natural hour.
	MaxSignaturesPerHour int `json:"maxSignaturesPerHour"`

	// MaxPayloadBytes caps the size of a single payload in bytes.
	MaxPayloadBytes int `json:"maxPayloadBytes"`

	// NotAfter is the instant after which the key may no longer be used.
	// A zero value means the key has no scheduled end of life.
	NotAfter time.Time `json:"notAfter"`

	maxSignaturesPerHourSet bool
	maxPayloadBytesSet      bool
}

// WithMaxSignaturesPerHour returns a copy of p with an explicit hourly
// signature cap. An explicit zero caps the key at zero signatures.
func (p Policy) WithMaxSignaturesPerHour(limit int) Policy {
	p.MaxSignaturesPerHour = limit
	p.maxSignaturesPerHourSet = true
	return p
}

// WithMaxPayloadBytes returns a copy of p with an explicit per-payload byte
// cap. An explicit zero only permits empty payloads.
func (p Policy) WithMaxPayloadBytes(limit int) Policy {
	p.MaxPayloadBytes = limit
	p.maxPayloadBytesSet = true
	return p
}

// Allows reports whether purpose may be used with this policy.
func (p Policy) Allows(purpose string) bool {
	for _, allowed := range p.Purposes {
		if purpose == allowed {
			return true
		}
	}
	return false
}

// PayloadAllowed reports whether a payload of size bytes fits the policy.
func (p Policy) PayloadAllowed(size int) bool {
	if size < 0 {
		return false
	}
	limit, limited := p.payloadLimit()
	if !limited {
		return true
	}
	return size <= limit
}

// SignaturesAllowed reports whether one more signature fits the policy after
// used signatures have already been issued in the current window.
func (p Policy) SignaturesAllowed(used int) bool {
	limit, limited := p.signatureLimit()
	if !limited {
		return true
	}
	return used < limit
}

// Expired reports whether the key has reached its NotAfter instant at now.
func (p Policy) Expired(now time.Time) bool {
	if p.NotAfter.IsZero() {
		return false
	}
	return !now.Before(p.NotAfter)
}

func (p Policy) signatureLimit() (int, bool) {
	return p.MaxSignaturesPerHour, p.maxSignaturesPerHourSet || p.MaxSignaturesPerHour != 0
}

func (p Policy) payloadLimit() (int, bool) {
	return p.MaxPayloadBytes, p.maxPayloadBytesSet || p.MaxPayloadBytes != 0
}

// MarshalJSON preserves whether a numeric limit was set explicitly so that an
// explicit zero is not mistaken for a permissive default after a reload.
func (p Policy) MarshalJSON() ([]byte, error) {
	type wire struct {
		Purposes             []string  `json:"purposes"`
		MaxSignaturesPerHour *int      `json:"maxSignaturesPerHour,omitempty"`
		MaxPayloadBytes      *int      `json:"maxPayloadBytes,omitempty"`
		NotAfter             time.Time `json:"notAfter"`
	}
	w := wire{Purposes: p.Purposes, NotAfter: p.NotAfter}
	if limit, limited := p.signatureLimit(); limited {
		w.MaxSignaturesPerHour = &limit
	}
	if limit, limited := p.payloadLimit(); limited {
		w.MaxPayloadBytes = &limit
	}
	return json.Marshal(w)
}

// UnmarshalJSON records which numeric limits were present in the input.
func (p *Policy) UnmarshalJSON(data []byte) error {
	type wire struct {
		Purposes             []string  `json:"purposes"`
		MaxSignaturesPerHour *int      `json:"maxSignaturesPerHour"`
		MaxPayloadBytes      *int      `json:"maxPayloadBytes"`
		NotAfter             time.Time `json:"notAfter"`
	}
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*p = Policy{
		Purposes: w.Purposes,
		NotAfter: w.NotAfter,
	}
	if w.MaxSignaturesPerHour != nil {
		p.MaxSignaturesPerHour = *w.MaxSignaturesPerHour
		p.maxSignaturesPerHourSet = true
	}
	if w.MaxPayloadBytes != nil {
		p.MaxPayloadBytes = *w.MaxPayloadBytes
		p.maxPayloadBytesSet = true
	}
	return nil
}

// HourWindow returns the start of the natural hour that contains t in t's
// location.
func HourWindow(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
}

// Counter tracks signature usage inside a single natural hour.
//
// The zero value is ready to use. A Counter is not safe for concurrent use;
// callers are expected to serialise access.
type Counter struct {
	window time.Time
	used   int
}

// Observe records one signature issued at now and returns the number of
// signatures recorded in the natural hour that contains now. Moving into a
// new hour starts a fresh window.
func (c *Counter) Observe(now time.Time) int {
	w := HourWindow(now)
	if !w.Equal(c.window) {
		c.window = w
		c.used = 0
	}
	c.used++
	return c.used
}

// Used returns the number of signatures recorded in the natural hour that
// contains now. An hour that has not seen any traffic reports zero.
func (c *Counter) Used(now time.Time) int {
	if !HourWindow(now).Equal(c.window) {
		return 0
	}
	return c.used
}
