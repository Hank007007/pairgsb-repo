// Package authz wires the DPoP proof check, replay protection, introspection
// and token binding into an HTTP middleware.
package authz

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"proofbind/internal/jwk"
	"proofbind/internal/proof"
	"proofbind/internal/replay"
	"proofbind/internal/token"
)

type contextKey struct{}

// Principal is the authenticated caller attached to the request context.
type Principal struct {
	Subject string
	Scope   string
	JKT     string
}

// Middleware authenticates requests carrying a DPoP-bound access token.
type Middleware struct {
	Tokens *token.Client
	Replay *replay.Cache
	Now    func() time.Time
	// ReplayTTL bounds how long an accepted proof identifier is remembered.
	ReplayTTL time.Duration
}

// New returns a middleware with the given introspection client.
func New(tokens *token.Client) *Middleware {
	return &Middleware{
		Tokens:    tokens,
		Replay:    replay.NewCache(5 * time.Minute),
		Now:       time.Now,
		ReplayTTL: 5 * time.Minute,
	}
}

// Handler wraps next so that only authenticated requests reach it.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := m.authenticate(r)
		if err != nil {
			challenge(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), principal)))
	})
}

// FromContext returns the principal attached by Handler.
func FromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(contextKey{}).(*Principal)
	return p, ok
}

// WithPrincipal returns a context carrying p.
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, p)
}

func (m *Middleware) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

func (m *Middleware) authenticate(r *http.Request) (*Principal, error) {
	accessToken, err := bearerToken(r)
	if err != nil {
		return nil, err
	}
	raw := r.Header.Get("DPoP")
	if raw == "" {
		return nil, errNoProof
	}

	now := m.now()
	pr, err := proof.Verify(raw, proof.Request{
		Method:      r.Method,
		Target:      requestURL(r),
		AccessToken: accessToken,
	})
	if err != nil {
		return nil, err
	}
	if err := proof.CheckFreshness(pr, now); err != nil {
		return nil, err
	}
	if pr.JTI == "" {
		return nil, errNoProof
	}

	jkt, err := jwk.Thumbprint(pr.Key)
	if err != nil {
		log.Printf("authz: thumbprint unavailable: %v", err)
	}

	replayTTL := m.ReplayTTL
	if pr.HasExp {
		until := time.Unix(pr.EXP, 0).Sub(now)
		if until > 0 && until < replayTTL {
			replayTTL = until
		}
	}
	if m.Replay != nil && m.Replay.MarkSeen(jkt, pr.JTI, now, replayTTL) {
		return nil, errReplay
	}

	info, err := m.Tokens.Introspect(r.Context(), accessToken)
	if err != nil {
		return nil, err
	}
	if info.CNFJKT != "" && jkt != "" && !jwk.Matches(info.CNFJKT, jkt) {
		return nil, errBinding
	}

	return &Principal{Subject: info.Subject, Scope: info.Scope, JKT: jkt}, nil
}

var (
	errNoProof = errors.New("authz: missing or malformed dpop proof")
	errReplay  = errors.New("authz: proof already used")
	errBinding = errors.New("authz: token is not bound to the presented key")
	errCreds   = errors.New("authz: missing or malformed credentials")
)

func bearerToken(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", errCreds
	}
	scheme, rest, ok := strings.Cut(auth, " ")
	if !ok || !strings.EqualFold(scheme, "DPoP") {
		return "", errCreds
	}
	rest = strings.TrimSpace(rest)
	if rest == "" || strings.ContainsAny(rest, " \t") {
		return "", errCreds
	}
	return rest, nil
}

func requestURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p == "http" || p == "https" {
		scheme = p
	}
	host := r.Host
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		host = h
	}
	return scheme + "://" + host + r.URL.Path
}

func challenge(w http.ResponseWriter, err error) {
	w.Header().Set("WWW-Authenticate", `DPoP error="invalid_dpop_proof"`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"invalid_request"}`))
}
