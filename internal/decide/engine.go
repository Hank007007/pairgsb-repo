// Package decide evaluates authorization requests, caching the outcomes.
package decide

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"permcache/internal/cache"
	"permcache/internal/policy"
)

// DefaultTTL is the upper bound on how long a decision may be cached,
// independent of the policy validity windows.
const DefaultTTL = time.Minute

// ErrEmptyField reports a request missing one of its identifying fields.
var ErrEmptyField = errors.New("decide: request field is empty")

// Request is one authorization question.
type Request struct {
	Tenant     string
	Subject    string
	Action     string
	Resource   string
	Attributes map[string]string
}

// Validate checks that all identifying fields are present.
func (r Request) Validate() error {
	switch {
	case r.Tenant == "":
		return fmt.Errorf("%w: tenant", ErrEmptyField)
	case r.Subject == "":
		return fmt.Errorf("%w: subject", ErrEmptyField)
	case r.Action == "":
		return fmt.Errorf("%w: action", ErrEmptyField)
	case r.Resource == "":
		return fmt.Errorf("%w: resource", ErrEmptyField)
	}
	return nil
}

// Decision is the outcome returned to the caller.
type Decision struct {
	Effect      cache.Effect
	MatchedRule string
	Revision    string
	Cached      bool
}

// Engine combines a policy loader with a decision cache. It is safe for
// concurrent use.
type Engine struct {
	loader *policy.Loader
	store  *cache.Store
	ttl    time.Duration
	now    func() time.Time

	mu       sync.Mutex
	revision string
}

// New returns an Engine backed by loader.
func New(loader *policy.Loader) *Engine {
	return &Engine{
		loader: loader,
		store:  cache.NewStore(),
		ttl:    DefaultTTL,
		now:    time.Now,
	}
}

// SetTTL bounds how long decisions may remain cached.
func (e *Engine) SetTTL(d time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ttl = d
}

// SetClock replaces the time source, which is useful for tests and for
// driving the engine from a simulated clock.
func (e *Engine) SetClock(now func() time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.now = now
}

// Store exposes the underlying cache for inspection and maintenance.
func (e *Engine) Store() *cache.Store {
	return e.store
}

// Decide returns the decision for req, consulting the cache first.
func (e *Engine) Decide(req Request) (Decision, error) {
	if err := req.Validate(); err != nil {
		return Decision{}, err
	}

	bundle, err := e.loader.Current()
	if err != nil {
		return Decision{}, err
	}

	e.mu.Lock()
	now := e.now
	ttl := e.ttl
	e.revision = bundle.Revision
	e.mu.Unlock()
	at := now()

	key := cache.BuildKey(req.Tenant, req.Subject, req.Action, req.Resource, req.Attributes, bundle.Revision)
	if entry, ok := e.store.Get(key, at); ok {
		return Decision{
			Effect:      entry.Effect,
			MatchedRule: entry.MatchedRule,
			Revision:    entry.Revision,
			Cached:      true,
		}, nil
	}

	effect, ruleID := policy.Evaluate(bundle, req.Tenant, req.Subject, req.Action, req.Resource, at)

	expires := at.Add(ttl)
	if boundary, ok := policy.NextBoundary(bundle.Rules, at); ok && boundary.Before(expires) {
		expires = boundary
	}

	e.store.Put(cache.Entry{
		Key:         key,
		Tenant:      req.Tenant,
		Subject:     req.Subject,
		Action:      req.Action,
		Resource:    req.Resource,
		Effect:      cache.Effect(effect),
		MatchedRule: ruleID,
		Revision:    bundle.Revision,
		ExpiresAt:   expires,
	})

	return Decision{
		Effect:      cache.Effect(effect),
		MatchedRule: ruleID,
		Revision:    bundle.Revision,
		Cached:      false,
	}, nil
}

// Revision returns the revision of the bundle used for the most recent
// decision, and whether any decision has been made yet.
func (e *Engine) Revision() (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.revision, e.revision != ""
}
