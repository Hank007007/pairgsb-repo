// Package tsa provides a local timestamp authority used by tests and by the
// beaconctl demo command.
package tsa

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Authority is an in-process timestamp authority. It signs every request it
// accepts with its private key and stamps the current time.
type Authority struct {
	pub  ed25519.PublicKey
	priv ed25519.PrivateKey

	mu     sync.Mutex
	off    bool
	now    func() time.Time
	stamps int
}

// NewAuthority creates an authority with a fresh key pair.
func NewAuthority() (*Authority, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &Authority{pub: pub, priv: priv, now: time.Now}, nil
}

var nowClock = time.Now

// PublicKey returns the key receipts are verified against.
func (a *Authority) PublicKey() ed25519.PublicKey { return a.pub }

// SetClock replaces the clock used to stamp tokens.
func (a *Authority) SetClock(now func() time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.now = now
}

// SetOffline simulates the authority being unreachable. While offline every
// request fails.
func (a *Authority) SetOffline(off bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.off = off
}

// Stamps reports how many tokens have been issued.
func (a *Authority) Stamps() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stamps
}

// Handler returns the HTTP handler exposing the /stamp endpoint.
func (a *Authority) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/stamp", a.handleStamp)
	return mux
}

func (a *Authority) handleStamp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	a.mu.Lock()
	off := a.off
	now := a.now
	a.mu.Unlock()
	if off {
		http.Error(w, "authority offline", http.StatusServiceUnavailable)
		return
	}

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.RootHex == "" || req.FromSeq <= 0 || req.ToSeq < req.FromSeq {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	genTime := now().UTC().Format(time.RFC3339)
	tok := Token{
		RootHex: req.RootHex,
		GenTime: genTime,
		Sig:     SignMessage(a.priv, req.RootHex, genTime),
	}

	a.mu.Lock()
	a.stamps++
	a.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tok)
}
