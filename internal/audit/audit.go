// Package audit records authorization decisions for operators and for the
// internal compliance pipeline. Each decision is emitted as one JSON object
// per line. Events never carry message payloads.
package audit

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// Decision values used in Event.Decision.
const (
	DecisionAllow = "allow"
	DecisionDeny  = "deny"
)

// Event is the external, stable shape of an audit record. Fields are added
// over time only at the end of this struct.
type Event struct {
	Time           time.Time `json:"time"`
	Decision       string    `json:"decision"`
	Action         string    `json:"action"`
	ClientID       string    `json:"client_id,omitempty"`
	Principal      string    `json:"principal,omitempty"`
	PrincipalClass string    `json:"principal_class,omitempty"`
	Filter         string    `json:"filter,omitempty"`
	Topic          string    `json:"topic,omitempty"`
	Tenant         string    `json:"tenant,omitempty"`
	RuleID         string    `json:"rule_id,omitempty"`
	Reason         string    `json:"reason,omitempty"`
}

// Logger writes audit events as newline-delimited JSON. It keeps the events it
// has written so that callers can inspect them in-process.
type Logger struct {
	mu     sync.Mutex
	w      io.Writer
	events []Event
}

// NewLogger returns a Logger that writes to w. A nil writer discards output
// while still retaining events in memory.
func NewLogger(w io.Writer) *Logger {
	return &Logger{w: w}
}

// Record writes a single event.
func (l *Logger) Record(ev Event) {
	if l == nil {
		return
	}
	if ev.Time.IsZero() {
		ev.Time = time.Now().UTC()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, ev)
	if l.w == nil {
		return
	}
	enc := json.NewEncoder(l.w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(ev)
}

// Deny records a denial.
func (l *Logger) Deny(ev Event) {
	ev.Decision = DecisionDeny
	l.Record(ev)
}

// Allow records an approval.
func (l *Logger) Allow(ev Event) {
	ev.Decision = DecisionAllow
	l.Record(ev)
}

// Events returns a copy of the events recorded so far.
func (l *Logger) Events() []Event {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Event, len(l.events))
	copy(out, l.events)
	return out
}
