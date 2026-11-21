package test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"proofbind/internal/authz"
	"proofbind/internal/token"
)

// TestAuthorizedRequest is the ordinary end-to-end flow: a fresh proof for
// this request, over an active token bound to the client key.
func TestAuthorizedRequest(t *testing.T) {
	var introspections atomic.Int64

	issuer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		introspections.Add(1)
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if got := r.Form.Get("token"); got != "access-token-1" {
			t.Errorf("introspected token = %q", got)
		}
		if err := r.ParseForm(); err == nil && r.Form.Get("token_type_hint") != "access_token" {
			t.Errorf("token_type_hint = %q", r.Form.Get("token_type_hint"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"active": true,
			"sub":    "alice",
			"scope":  "documents.read",
			"exp":    time.Now().Add(5 * time.Minute).Unix(),
			"cnf":    map[string]any{"jkt": clientJKT},
		})
	}))
	defer issuer.Close()

	client := token.NewClient(issuer.URL, "resource-server", "s3cret")
	auth := authz.New(client)

	handler := auth.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authz.FromContext(r.Context())
		if !ok {
			t.Error("handler ran without a principal")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"subject": principal.Subject,
			"scope":   principal.Scope,
		})
	}))

	server := httptest.NewServer(handler)
	defer server.Close()

	now := unixNow()
	proof := signProof(t, proofOptions{
		Method:      http.MethodGet,
		URL:         server.URL + "/api/whoami",
		AccessToken: "access-token-1",
		JTI:         newJTI(t),
		IAT:         now,
		EXP:         now + 120,
	})

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/whoami", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "DPoP access-token-1")
	req.Header.Set("DPoP", proof)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}

	var body struct {
		Subject string `json:"subject"`
		Scope   string `json:"scope"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Subject != "alice" {
		t.Errorf("subject = %q, want alice", body.Subject)
	}
	if body.Scope != "documents.read" {
		t.Errorf("scope = %q, want documents.read", body.Scope)
	}
	if got := introspections.Load(); got != 1 {
		t.Errorf("introspection calls = %d, want 1", got)
	}
}
