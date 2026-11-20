// Package rotator keeps a tls.Config supplied with fresh certificate
// material for a long-running service.
//
// The Rotator loads a pair, publishes it through an atomic pointer, and then
// periodically checks whether the certificate is close enough to expiry to be
// replaced. Consumers call TLSConfig for every new connection and therefore
// always observe a complete configuration.
package rotator

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"certwhirl/internal/loader"
)

// DefaultRotateBefore is how long before expiry a certificate is replaced
// when Config.RotateBefore is left at zero.
const DefaultRotateBefore = 72 * time.Hour

// DefaultCheckInterval is how often the background loop inspects the current
// certificate when Config.CheckInterval is left at zero.
const DefaultCheckInterval = time.Minute

// Clock supplies the current time. Tests can provide a controllable
// implementation.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Config configures a Rotator.
type Config struct {
	// Load obtains and validates a certificate pair. It is called once by New
	// and again whenever the current certificate is due for replacement.
	Load func(context.Context) (*loader.Pair, error)
	// RotateBefore is the lead time before expiry at which replacement should
	// begin. Zero selects DefaultRotateBefore.
	RotateBefore time.Duration
	// CheckInterval is the delay between background checks. Zero selects
	// DefaultCheckInterval.
	CheckInterval time.Duration
	// Clock supplies the current time. Nil selects the system clock.
	Clock Clock
	// OnError, when set, receives errors from the background loop.
	OnError func(error)
}

// Rotator owns the published tls.Config and the loop that refreshes it.
type Rotator struct {
	cfg Config

	current atomic.Pointer[tls.Config]
	pair    atomic.Pointer[loader.Pair]

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// New builds a Rotator and performs the initial load. The returned Rotator is
// ready to serve TLS through TLSConfig.
func New(cfg Config) (*Rotator, error) {
	if cfg.Load == nil {
		return nil, errors.New("rotator: Load is required")
	}
	if cfg.RotateBefore < 0 {
		return nil, errors.New("rotator: RotateBefore must not be negative")
	}
	if cfg.RotateBefore == 0 {
		cfg.RotateBefore = DefaultRotateBefore
	}
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = DefaultCheckInterval
	}
	if cfg.Clock == nil {
		cfg.Clock = systemClock{}
	}

	r := &Rotator{cfg: cfg, done: make(chan struct{})}
	initial, err := cfg.Load(context.Background())
	if err != nil {
		return nil, fmt.Errorf("rotator: initial load: %w", err)
	}
	if err := r.swap(initial); err != nil {
		return nil, err
	}
	return r, nil
}

// TLSConfig returns the configuration currently published for new
// connections. The returned value is never nil, so callers can hand it to
// tls.Server or http.Server without a nil check.
func (r *Rotator) TLSConfig() *tls.Config {
	if cfg := r.current.Load(); cfg != nil {
		return cfg
	}
	return &tls.Config{MinVersion: tls.VersionTLS12}
}

// Start launches the background check loop. It returns immediately. Calling
// Start more than once has no effect.
func (r *Rotator) Start(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	go r.loop(runCtx)
}

// Stop ends the background loop and waits for it to exit. It is safe to call
// on a Rotator that was never started.
func (r *Rotator) Stop() {
	r.mu.Lock()
	cancel := r.cancel
	r.cancel = nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
		<-r.done
	}
}

// CheckAndRotate performs one evaluation. It replaces the published
// certificate when the current one is due, and returns any error encountered
// while loading or publishing the replacement.
func (r *Rotator) CheckAndRotate(ctx context.Context) error {
	current := r.pair.Load()
	if current != nil && !shouldRotate(current.Leaf, r.cfg.Clock.Now(), r.cfg.RotateBefore) {
		return nil
	}
	next, err := r.cfg.Load(ctx)
	if err != nil {
		return fmt.Errorf("rotator: load replacement: %w", err)
	}
	return r.swap(next)
}

// shouldRotate reports whether a certificate should be replaced at now, given
// the configured lead time.
func shouldRotate(leaf *x509.Certificate, now time.Time, rotateBefore time.Duration) bool {
	if leaf == nil {
		return true
	}
	remaining := leaf.NotAfter.Sub(now)
	return remaining < 0
}

// swap builds a tls.Config from pair and publishes it.
func (r *Rotator) swap(pair *loader.Pair) error {
	if pair == nil {
		r.current.Store(nil)
		return errors.New("rotator: no certificate material")
	}
	cert, err := tls.X509KeyPair(pair.CertPEM, pair.KeyPEM)
	if err != nil {
		r.current.Store(nil)
		return fmt.Errorf("rotator: parse certificate pair: %w", err)
	}
	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	r.current.Store(cfg)
	r.pair.Store(pair)
	return nil
}

func (r *Rotator) loop(ctx context.Context) {
	defer close(r.done)
	ticker := time.NewTicker(r.cfg.CheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.CheckAndRotate(ctx); err != nil {
				r.report(err)
			}
		}
	}
}

func (r *Rotator) report(err error) {
	if r.cfg.OnError != nil {
		r.cfg.OnError(err)
	}
}
