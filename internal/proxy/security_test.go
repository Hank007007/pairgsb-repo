package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"hopfence/internal/config"
)

func TestDenyPathsMatchOnSegmentBoundaries(t *testing.T) {
	upstream, _ := newUpstream(t, "ok")
	h, _ := newHandler(t, []config.Route{
		{
			Name:      "legacy",
			Prefix:    "/legacy",
			Upstream:  upstream.URL,
			DenyPaths: []string{"/legacy/admin"},
		},
	})

	for _, tc := range []struct {
		path string
		want int
	}{
		{"/legacy/admin", http.StatusForbidden},
		{"/legacy/admin/ops", http.StatusForbidden},
		{"/legacy/admin-backup", http.StatusOK},
		{"/legacy/adminpanel", http.StatusOK},
	} {
		rec := serve(t, h, httptest.NewRequest("GET", "http://hopfence.local"+tc.path, nil))
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", tc.path, rec.Code, tc.want)
		}
	}
}

func TestEncodedDotDotCannotEscapeRoutePrefix(t *testing.T) {
	var staticHits, adminHits atomic.Int32
	static := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		staticHits.Add(1)
	}))
	defer static.Close()
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		adminHits.Add(1)
	}))
	defer admin.Close()

	h, _ := newHandler(t, []config.Route{
		{Name: "static", Prefix: "/static", Upstream: static.URL},
		{Name: "admin", Prefix: "/admin", Upstream: admin.URL},
	})

	req := httptest.NewRequest("GET", "http://hopfence.local/static/%2e%2e/admin", nil)
	rec := serve(t, h, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if staticHits.Load() != 0 || adminHits.Load() != 0 {
		t.Fatalf("request reached upstream: static=%d admin=%d", staticHits.Load(), adminHits.Load())
	}
}

func TestPathIsDecodedOnlyOnce(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
	}))
	defer upstream.Close()

	h, _ := newHandler(t, []config.Route{
		{Name: "static", Prefix: "/static", Upstream: upstream.URL},
	})

	req := httptest.NewRequest("GET", "http://hopfence.local/static/%252e%252e/admin", nil)
	if rec := serve(t, h, req); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if gotPath != "/static/%2e%2e/admin" {
		t.Errorf("upstream path = %q, want one decode to /static/%%2e%%2e/admin", gotPath)
	}
}

func TestCleanedPathIsForwardedAndLogged(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
	}))
	defer upstream.Close()

	h, logs := newHandler(t, []config.Route{
		{Name: "api", Prefix: "/api", Upstream: upstream.URL},
	})
	rec := serve(t, h, httptest.NewRequest("GET", "http://hopfence.local/api/a/../users?page=2", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if gotPath != "/api/users" {
		t.Errorf("upstream path = %q, want /api/users", gotPath)
	}
	if !strings.Contains(logs.String(), " /api/users ") {
		t.Errorf("log = %q, want cleaned path", logs.String())
	}
}

func TestClientAddressComesFromTCPPeer(t *testing.T) {
	var seen *http.Request
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Clone(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	h, logs := newHandler(t, []config.Route{
		{Name: "admin", Prefix: "/admin", Upstream: upstream.URL, Allow: []string{"10.0.0.0/8"}},
	})

	req := httptest.NewRequest("GET", "http://hopfence.local/admin/panel", nil)
	req.RemoteAddr = "192.0.2.10:5555"
	req.Header.Set("X-Forwarded-For", "10.1.2.3")
	rec := serve(t, h, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("spoofed X-Forwarded-For bypassed ACL: status = %d", rec.Code)
	}
	if !strings.HasPrefix(logs.String(), "192.0.2.10 ") {
		t.Errorf("log = %q, want TCP peer address", logs.String())
	}

	req = httptest.NewRequest("GET", "http://hopfence.local/admin/panel", nil)
	req.RemoteAddr = "10.1.2.3:5555"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Real-IP", "1.2.3.4")
	rec = serve(t, h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := seen.Header.Values("X-Forwarded-For"); len(got) != 1 || got[0] != "10.1.2.3" {
		t.Errorf("X-Forwarded-For = %q, want [10.1.2.3]", got)
	}
	if got := seen.Header.Get("X-Real-IP"); got != "10.1.2.3" {
		t.Errorf("X-Real-IP = %q, want 10.1.2.3", got)
	}
}

func TestHopByHopRequestHeadersAreRemoved(t *testing.T) {
	var seen *http.Request
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Clone(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	h, _ := newHandler(t, []config.Route{
		{Name: "api", Prefix: "/api", Upstream: upstream.URL},
	})

	req := httptest.NewRequest("GET", "http://hopfence.local/api/hdr", nil)
	req.Header.Set("Connection", "X-Admin")
	req.Header.Set("X-Admin", "1")
	req.Header.Set("Keep-Alive", "timeout=5")
	req.Header.Set("Proxy-Authorization", "Basic eA==")
	req.Header.Set("X-Business", "kept")
	if rec := serve(t, h, req); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	for _, name := range []string{"Connection", "X-Admin", "Keep-Alive", "Proxy-Authorization"} {
		if got := seen.Header.Values(name); len(got) != 0 {
			t.Errorf("%s leaked upstream: %q", name, got)
		}
	}
	if got := seen.Header.Get("X-Business"); got != "kept" {
		t.Errorf("X-Business = %q, want kept", got)
	}
}
