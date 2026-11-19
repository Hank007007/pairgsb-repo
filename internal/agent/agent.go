// Package agent is the entry point of the signing proxy. Callers hold an
// opaque key handle and ask for an Ed25519 signature; key material never
// leaves this process.
//
// The rules that govern when a signature may be issued are described in
// docs/agent-policy.md.
package agent

import (
	"errors"
	"sync"
	"time"

	"signward/internal/audit"
	"signward/internal/keystore"
	"signward/internal/policy"
)

// SessionTTL is the validity window of a signing session opened by Unlock.
const SessionTTL = 15 * time.Minute

// Errors returned by the agent.
var (
	ErrSessionLocked    = errors.New("agent: session is locked")
	ErrPurposeDenied    = errors.New("agent: purpose not allowed for key")
	ErrPayloadTooLarge  = errors.New("agent: payload exceeds policy limit")
	ErrQuotaExceeded    = errors.New("agent: hourly signature quota exceeded")
	ErrKeyHandleMissing = errors.New("agent: key handle is required")
)

// Request is a single signing request.
type Request struct {
	// KeyHandle identifies the key to sign with.
	KeyHandle string
	// Purpose is the declared use of the signature.
	Purpose string
	// Payload is the message to sign.
	Payload []byte
}

// Response is a completed signing request.
type Response struct {
	KeyHandle string
	Purpose   string
	Signature []byte
	IssuedAt  time.Time
}

// Option configures an Agent.
type Option func(*Agent)

// WithClock replaces the agent's source of time. It is intended for tests.
func WithClock(now func() time.Time) Option {
	return func(a *Agent) {
		if now != nil {
			a.now = now
		}
	}
}

// Agent issues signatures under policy and records what it did.
type Agent struct {
	mu     sync.Mutex
	store  *keystore.Store
	audit  *audit.Log
	now    func() time.Time
	unlock bool

	unlockedAt time.Time
	expiresAt  time.Time

	counters map[string]*policy.Counter
}

// New returns an Agent backed by store. audit may be nil.
func New(store *keystore.Store, log *audit.Log, opts ...Option) *Agent {
	a := &Agent{
		store:    store,
		audit:    log,
		now:      time.Now,
		counters: make(map[string]*policy.Counter),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Unlock opens a signing session for the calling build step.
func (a *Agent) Unlock() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := a.now()
	a.unlock = true
	a.unlockedAt = now
	a.expiresAt = time.Time{}
	a.audit.RecordAt(now, audit.EventSessionUnlocked, "", "", "")
	return nil
}

// Lock closes the current signing session.
func (a *Agent) Lock() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.unlock = false
	a.unlockedAt = time.Time{}
	a.expiresAt = time.Time{}
	a.audit.RecordAt(a.now(), audit.EventSessionLocked, "", "", "")
	return nil
}

// Locked reports whether a signing session is available at the current time.
func (a *Agent) Locked() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return !a.sessionActive(a.now())
}

// Sign validates req against the session and the key policy and, when it is
// allowed, returns the signature.
func (a *Agent) Sign(req Request) (Response, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := a.now()

	if !a.sessionActive(now) {
		a.audit.RecordAt(now, audit.EventSignDenied, req.KeyHandle, req.Purpose, "session locked")
		return Response{}, ErrSessionLocked
	}
	if req.KeyHandle == "" {
		a.audit.RecordAt(now, audit.EventSignDenied, "", req.Purpose, "missing key handle")
		return Response{}, ErrKeyHandleMissing
	}

	key, err := a.store.Resolve(req.KeyHandle)
	if err != nil {
		a.audit.RecordAt(now, audit.EventSignDenied, req.KeyHandle, req.Purpose, err.Error())
		return Response{}, err
	}
	if !key.Policy.Allows(req.Purpose) {
		a.audit.RecordAt(now, audit.EventSignDenied, req.KeyHandle, req.Purpose, "purpose not allowed")
		return Response{}, ErrPurposeDenied
	}
	if !key.Policy.PayloadAllowed(len(req.Payload)) {
		a.audit.RecordAt(now, audit.EventSignDenied, req.KeyHandle, req.Purpose, "payload too large")
		return Response{}, ErrPayloadTooLarge
	}

	counter := a.counterFor(key.Handle)
	sig := key.Sign(req.Payload)
	used := counter.Observe(now)
	if used > key.Policy.MaxSignaturesPerHour {
		a.audit.RecordAt(now, audit.EventSignDenied, req.KeyHandle, req.Purpose, "hourly signature quota exceeded")
	}

	a.audit.RecordAt(now, audit.EventSignIssued, req.KeyHandle, req.Purpose, "")
	return Response{
		KeyHandle: key.Handle,
		Purpose:   req.Purpose,
		Signature: sig,
		IssuedAt:  now,
	}, nil
}

// sessionActive reports whether Sign may proceed at now. An expired session
// is closed and recorded.
func (a *Agent) sessionActive(now time.Time) bool {
	if !a.unlock {
		return false
	}
	if a.expiresAt.IsZero() {
		return true
	}
	if !now.Before(a.expiresAt) {
		a.unlock = false
		a.audit.RecordAt(now, audit.EventSessionExpired, "", "", "")
		return false
	}
	return true
}

func (a *Agent) counterFor(handle string) *policy.Counter {
	c, ok := a.counters[handle]
	if !ok {
		c = new(policy.Counter)
		a.counters[handle] = c
	}
	return c
}
