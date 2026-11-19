// Package api wires the HTTP surface of the broker.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"pathvault/internal/audit"
	"pathvault/internal/lease"
	"pathvault/internal/policy"
	"pathvault/internal/store"
	"pathvault/internal/token"
)

// Server holds the dependencies of the HTTP handlers.
type Server struct {
	Policy     *policy.Policy
	Secrets    *store.KV
	Leases     *lease.Manager
	Tokens     *token.Manager
	Audit      *audit.Logger
	AdminToken string
}

// New returns a Server with the given dependencies.
func New(p *policy.Policy, kv *store.KV, lm *lease.Manager, tm *token.Manager, al *audit.Logger, adminToken string) *Server {
	return &Server{
		Policy:     p,
		Secrets:    kv,
		Leases:     lm,
		Tokens:     tm,
		Audit:      al,
		AdminToken: adminToken,
	}
}

// Handler returns the root mux.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /v1/secret/{path...}", s.handleRead)
	mux.HandleFunc("POST /v1/wrap", s.handleWrap)
	mux.HandleFunc("POST /v1/unwrap", s.handleUnwrap)
	mux.HandleFunc("POST /v1/backends/{backend}/revoke", s.handleRevoke)
	return mux
}

type errorBody struct {
	Error string `json:"error"`
}

type secretBody struct {
	Path    string `json:"path"`
	Value   string `json:"value"`
	LeaseID string `json:"lease_id"`
	Backend string `json:"backend"`
}

type wrapBody struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

type wrapRequest struct {
	Path string `json:"path"`
	TTL  int64  `json:"ttl"`
}

type unwrapRequest struct {
	Token string `json:"token"`
}

type revokeBody struct {
	Backend string `json:"backend"`
	Revoked int    `json:"revoked"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorBody{Error: msg})
}

// bearer extracts the token from an Authorization header.
func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

func (s *Server) record(e audit.Event) {
	if s.Audit == nil {
		return
	}
	_ = s.Audit.Record(e)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	accessorToken := bearer(r)
	acc, ok := s.Policy.ByToken(accessorToken)
	if !ok {
		s.record(audit.Event{Path: path, Action: "read", Decision: "error"})
		writeError(w, http.StatusUnauthorized, "invalid accessor token")
		return
	}

	if leaseID := r.Header.Get("X-Lease-Id"); leaseID != "" {
		s.serveLeaseRead(w, acc.ID, path, leaseID)
		return
	}

	if !acc.Evaluate(path) {
		s.record(audit.Event{Accessor: acc.ID, Path: path, Action: "read", Decision: "deny"})
		writeError(w, http.StatusNotFound, "secret not found")
		return
	}
	secret, ok := s.Secrets.Get(path)
	if !ok {
		s.record(audit.Event{Accessor: acc.ID, Path: path, Action: "read", Decision: "deny"})
		writeError(w, http.StatusNotFound, "secret not found")
		return
	}

	l := s.Leases.Issue(acc.ID, path, secret.Backend)
	s.Leases.CacheValue(secret.Backend, path, secret.Value)
	s.record(audit.Event{
		Accessor: acc.ID,
		Path:     path,
		Action:   "read",
		Decision: "allow",
		LeaseID:  l.ID,
		Backend:  secret.Backend,
	})
	writeJSON(w, http.StatusOK, secretBody{
		Path:    path,
		Value:   secret.Value,
		LeaseID: l.ID,
		Backend: secret.Backend,
	})
}

func (s *Server) serveLeaseRead(w http.ResponseWriter, accessor, path, leaseID string) {
	l, err := s.Leases.Check(leaseID, accessor, path)
	if err != nil {
		decision := "error"
		if errors.Is(err, lease.ErrRevoked) {
			decision = "deny"
		}
		s.record(audit.Event{
			Accessor: accessor,
			Path:     path,
			Action:   "read",
			Decision: decision,
			LeaseID:  leaseID,
		})
		writeError(w, http.StatusForbidden, "lease revoked")
		return
	}
	value, ok := s.Leases.CachedValue(l.Backend, path)
	if !ok {
		s.record(audit.Event{
			Accessor: accessor,
			Path:     path,
			Action:   "read",
			Decision: "deny",
			LeaseID:  leaseID,
			Backend:  l.Backend,
		})
		writeError(w, http.StatusForbidden, "lease revoked")
		return
	}
	s.record(audit.Event{
		Accessor: accessor,
		Path:     path,
		Action:   "read",
		Decision: "allow",
		LeaseID:  leaseID,
		Backend:  l.Backend,
	})
	writeJSON(w, http.StatusOK, secretBody{
		Path:    path,
		Value:   value,
		LeaseID: l.ID,
		Backend: l.Backend,
	})
}

func (s *Server) handleWrap(w http.ResponseWriter, r *http.Request) {
	accessorToken := bearer(r)
	acc, ok := s.Policy.ByToken(accessorToken)
	if !ok {
		s.record(audit.Event{Action: "wrap", Decision: "error"})
		writeError(w, http.StatusUnauthorized, "invalid accessor token")
		return
	}
	var req wrapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.record(audit.Event{Accessor: acc.ID, Action: "wrap", Decision: "error"})
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Path == "" {
		s.record(audit.Event{Accessor: acc.ID, Action: "wrap", Decision: "error"})
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	if !acc.Evaluate(req.Path) {
		s.record(audit.Event{Accessor: acc.ID, Path: req.Path, Action: "wrap", Decision: "deny"})
		writeError(w, http.StatusNotFound, "secret not found")
		return
	}
	if _, ok := s.Secrets.Get(req.Path); !ok {
		s.record(audit.Event{Accessor: acc.ID, Path: req.Path, Action: "wrap", Decision: "deny"})
		writeError(w, http.StatusNotFound, "secret not found")
		return
	}
	wrap, err := s.Tokens.Mint(acc.ID, req.Path, req.TTL)
	if err != nil {
		s.record(audit.Event{Accessor: acc.ID, Path: req.Path, Action: "wrap", Decision: "error"})
		writeError(w, http.StatusInternalServerError, "could not mint token")
		return
	}
	s.record(audit.Event{
		Accessor: acc.ID,
		Path:     req.Path,
		Action:   "wrap",
		Decision: "allow",
	})
	writeJSON(w, http.StatusCreated, wrapBody{
		Token:     wrap.Value,
		ExpiresAt: wrap.ExpiresAt.UTC().Format(time.RFC3339Nano),
	})
}

func (s *Server) handleUnwrap(w http.ResponseWriter, r *http.Request) {
	var req unwrapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.record(audit.Event{Action: "unwrap", Decision: "error"})
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	wrap, err := s.Tokens.Unwrap(req.Token)
	if err != nil {
		s.record(audit.Event{Action: "unwrap", Decision: "deny"})
		writeError(w, http.StatusNotFound, "token not found")
		return
	}
	secret, ok := s.Secrets.Get(wrap.Path)
	if !ok {
		s.record(audit.Event{Accessor: wrap.Accessor, Path: wrap.Path, Action: "unwrap", Decision: "deny"})
		writeError(w, http.StatusNotFound, "secret not found")
		return
	}
	l := s.Leases.Issue(wrap.Accessor, wrap.Path, secret.Backend)
	s.Leases.CacheValue(secret.Backend, wrap.Path, secret.Value)
	s.record(audit.Event{
		Accessor: wrap.Accessor,
		Path:     wrap.Path,
		Action:   "unwrap",
		Decision: "allow",
		LeaseID:  l.ID,
		Backend:  secret.Backend,
	})
	writeJSON(w, http.StatusOK, secretBody{
		Path:    wrap.Path,
		Value:   secret.Value,
		LeaseID: l.ID,
		Backend: secret.Backend,
	})
}

func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	backend := r.PathValue("backend")
	if bearer(r) != s.AdminToken || s.AdminToken == "" {
		s.record(audit.Event{Action: "revoke", Decision: "error", Backend: backend})
		writeError(w, http.StatusUnauthorized, "invalid admin token")
		return
	}
	if !s.backendExists(backend) {
		s.record(audit.Event{Action: "revoke", Decision: "deny", Backend: backend})
		writeError(w, http.StatusNotFound, "unknown backend")
		return
	}
	n := s.Leases.Revoke(backend)
	s.record(audit.Event{
		Action:   "revoke",
		Decision: "allow",
		Backend:  backend,
	})
	writeJSON(w, http.StatusOK, revokeBody{Backend: backend, Revoked: n})
}

func (s *Server) backendExists(backend string) bool {
	for _, p := range s.Secrets.Paths() {
		if secret, ok := s.Secrets.Get(p); ok && secret.Backend == backend {
			return true
		}
	}
	return false
}
