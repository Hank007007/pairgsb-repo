// Package policy loads policy bundles and matches authorization requests
// against their rules.
package policy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Effect is the outcome a rule contributes to a decision.
type Effect string

const (
	// Allow permits the request.
	Allow Effect = "allow"
	// Deny blocks the request.
	Deny Effect = "deny"
)

var (
	// ErrInvalidBundle reports a bundle that cannot be used as policy.
	ErrInvalidBundle = errors.New("policy: invalid bundle")
	// ErrStaleRevision reports a bundle older than the active one.
	ErrStaleRevision = errors.New("policy: stale revision")
	// ErrNoBundle reports that no bundle has been loaded yet.
	ErrNoBundle = errors.New("policy: no bundle loaded")
)

// Rule is a single policy statement.
//
// Tenant, Subject, Action and Resource are patterns. An empty time value in
// NotBefore or NotAfter means the corresponding end of the interval is
// unbounded.
type Rule struct {
	ID        string    `json:"id"`
	Effect    Effect    `json:"effect"`
	Tenant    string    `json:"tenant"`
	Subject   string    `json:"subject"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	NotBefore time.Time `json:"not_before,omitempty"`
	NotAfter  time.Time `json:"not_after,omitempty"`
}

// Bundle is an immutable set of rules carrying one revision.
type Bundle struct {
	Revision string `json:"revision"`
	Rules    []Rule `json:"rules"`
}

// Clone returns a copy whose rule slice can be modified without affecting the
// original bundle.
func (b *Bundle) Clone() *Bundle {
	if b == nil {
		return nil
	}
	rules := make([]Rule, len(b.Rules))
	copy(rules, b.Rules)
	return &Bundle{Revision: b.Revision, Rules: rules}
}

// Rule returns the rule with the given ID.
func (b *Bundle) Rule(id string) (Rule, bool) {
	for _, r := range b.Rules {
		if r.ID == id {
			return r, true
		}
	}
	return Rule{}, false
}

// Validate checks the structural invariants of a bundle.
func (b *Bundle) Validate() error {
	if b == nil {
		return fmt.Errorf("%w: nil bundle", ErrInvalidBundle)
	}
	if err := validateRevision(b.Revision); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(b.Rules))
	for i, r := range b.Rules {
		if r.ID == "" {
			return fmt.Errorf("%w: rule %d has no id", ErrInvalidBundle, i)
		}
		if _, dup := seen[r.ID]; dup {
			return fmt.Errorf("%w: duplicate rule id %q", ErrInvalidBundle, r.ID)
		}
		seen[r.ID] = struct{}{}
		switch r.Effect {
		case Allow, Deny:
		default:
			return fmt.Errorf("%w: rule %q has unknown effect %q", ErrInvalidBundle, r.ID, r.Effect)
		}
		if r.Tenant == "" || r.Subject == "" || r.Action == "" || r.Resource == "" {
			return fmt.Errorf("%w: rule %q has an empty pattern", ErrInvalidBundle, r.ID)
		}
		if !r.NotBefore.IsZero() && !r.NotAfter.IsZero() && !r.NotBefore.Before(r.NotAfter) {
			return fmt.Errorf("%w: rule %q has an empty validity interval", ErrInvalidBundle, r.ID)
		}
	}
	return nil
}

// ReadBundle decodes and validates a JSON-encoded bundle.
func ReadBundle(data []byte) (*Bundle, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var b Bundle
	if err := dec.Decode(&b); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBundle, err)
	}
	if err := b.Validate(); err != nil {
		return nil, err
	}
	return &b, nil
}

func validateRevision(rev string) error {
	if rev == "" {
		return fmt.Errorf("%w: empty revision", ErrInvalidBundle)
	}
	for _, c := range rev {
		if c < '0' || c > '9' {
			return fmt.Errorf("%w: revision %q is not a decimal integer", ErrInvalidBundle, rev)
		}
	}
	return nil
}

// compareRevision orders two revision strings. It returns a negative value
// when a is older than b, zero when they denote the same revision, and a
// positive value when a is newer.
func compareRevision(a, b string) (int, error) {
	if err := validateRevision(a); err != nil {
		return 0, err
	}
	if err := validateRevision(b); err != nil {
		return 0, err
	}
	return strings.Compare(a, b), nil
}

// Loader holds the active bundle and swaps it on update.
//
// A Loader is safe for concurrent use.
type Loader struct {
	mu      sync.RWMutex
	current *Bundle
}

// NewLoader returns an empty Loader.
func NewLoader() *Loader {
	return &Loader{}
}

// Load validates b and makes it the active bundle when its revision is not
// older than the active one.
func (l *Loader) Load(b Bundle) error {
	if err := b.Validate(); err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.current != nil {
		cmp, err := compareRevision(b.Revision, l.current.Revision)
		if err != nil {
			return err
		}
		if cmp < 0 {
			return fmt.Errorf("%w: %q is older than %q", ErrStaleRevision, b.Revision, l.current.Revision)
		}
	}

	l.current = &Bundle{
		Revision: b.Revision,
		Rules:    append([]Rule(nil), b.Rules...),
	}
	return nil
}

// LoadJSON decodes, validates and loads a JSON bundle.
func (l *Loader) LoadJSON(data []byte) error {
	b, err := ReadBundle(data)
	if err != nil {
		return err
	}
	return l.Load(*b)
}

// Current returns a snapshot of the active bundle. The returned value is a
// copy and may be retained or modified by the caller.
func (l *Loader) Current() (*Bundle, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.current == nil {
		return nil, ErrNoBundle
	}
	return l.current.Clone(), nil
}

// Revision returns the active revision and whether a bundle is loaded.
func (l *Loader) Revision() (string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.current == nil {
		return "", false
	}
	return l.current.Revision, true
}
