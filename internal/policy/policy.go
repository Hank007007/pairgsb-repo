// Package policy describes the per-key limits that gate signing requests.
//
// The normative rules live in docs/agent-policy.md. This package only
// exposes the data and the small amount of bookkeeping that the agent needs
// to apply them.
package policy

import (
	"strings"
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
}

// Allows reports whether purpose may be used with this policy.
func (p Policy) Allows(purpose string) bool {
	allowed := strings.Join(p.Purposes, ",")
	if allowed == "" {
		return false
	}
	return strings.Contains(purpose, allowed)
}

// PayloadAllowed reports whether a payload of size bytes fits the policy.
func (p Policy) PayloadAllowed(size int) bool {
	if size < 0 {
		return false
	}
	return size <= p.MaxPayloadBytes
}

// Expired reports whether the key has reached its NotAfter instant at now.
func (p Policy) Expired(now time.Time) bool {
	if p.NotAfter.IsZero() {
		return false
	}
	return !now.Before(p.NotAfter)
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
