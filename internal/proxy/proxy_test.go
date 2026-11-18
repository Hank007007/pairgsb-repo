package proxy

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hopfence/internal/accesslog"
	"hopfence/internal/config"
)

// newUpstream starts a test upstream that records the last request and replies
// with body.
func newUpstream(t *testing.T, body string) (*httptest.Server, *http.Request) {
	t.Helper()
	var seen *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Clone(r.Context())
		seen.Body = io.NopCloser(bytes.NewReader(nil))
		w.Header().Set("X-Upstream", "yes")
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, seen
}

func newHandler(t *testing.T, routes []config.Route) (*Handler, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	cfg := config.Config{
		Listen:         "127.0.0.1:0",
		TimeoutSeconds: 5,
		Routes:         routes,
	}
	h, err := New(cfg, accesslog.New(&buf))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h, &buf
}

func serve(t *testing.T, h *Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestForwardsToMatchingUpstream(t *testing.T) {
	upstream, _ := newUpstream(t, "hello from upstream")
	h, logs := newHandler(t, []config.Route{
		{Name: "api", Prefix: "/api", Upstream: upstream.URL},
	})

	req := httptest.NewRequest("GET", "http://hopfence.local/api/users?page=2", nil)
	req.Header.Set("X-Request-Id", "abc123")
	rec := serve(t, h, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "hello from upstream" {
		t.Errorf("body = %q", got)
	}
	if got := rec.Header().Get("X-Upstream"); got != "yes" {
		t.Errorf("X-Upstream = %q, want yes", got)
	}
	if logs.Len() == 0 {
		t.Fatal("no access log line written")
	}
	if !strings.Contains(logs.String(), "/api/users") {
		t.Errorf("log = %q, want path", logs.String())
	}
}

func TestUpstreamReceivesPathAndQuery(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	h, _ := newHandler(t, []config.Route{
		{Name: "api", Prefix: "/api", Upstream: srv.URL},
	})
	req := httptest.NewRequest("GET", "http://hopfence.local/api/users?page=2&sort=asc", nil)
	serve(t, h, req)

	if gotPath != "/api/users" {
		t.Errorf("upstream path = %q, want /api/users", gotPath)
	}
	if gotQuery != "page=2&sort=asc" {
		t.Errorf("upstream query = %q", gotQuery)
	}
}

func TestLongestPrefixWins(t *testing.T) {
	root, _ := newUpstream(t, "root")
	special, _ := newUpstream(t, "special")
	h, _ := newHandler(t, []config.Route{
		{Name: "root", Prefix: "/", Upstream: root.URL},
		{Name: "special", Prefix: "/special", Upstream: special.URL},
	})

	rec := serve(t, h, httptest.NewRequest("GET", "http://hopfence.local/special/page", nil))
	if got := rec.Body.String(); got != "special" {
		t.Fatalf("body = %q, want special", got)
	}

	rec = serve(t, h, httptest.NewRequest("GET", "http://hopfence.local/other", nil))
	if got := rec.Body.String(); got != "root" {
		t.Fatalf("body = %q, want root", got)
	}
}

func TestUnknownPathReturnsNotFound(t *testing.T) {
	upstream, _ := newUpstream(t, "x")
	h, _ := newHandler(t, []config.Route{
		{Name: "api", Prefix: "/api", Upstream: upstream.URL},
	})
	rec := serve(t, h, httptest.NewRequest("GET", "http://hopfence.local/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestDeniedClientReturnsForbidden(t *testing.T) {
	upstream, _ := newUpstream(t, "x")
	h, _ := newHandler(t, []config.Route{
		{Name: "admin", Prefix: "/admin", Upstream: upstream.URL, Allow: []string{"10.0.0.0/8"}},
	})
	req := httptest.NewRequest("GET", "http://hopfence.local/admin", nil)
	req.RemoteAddr = "192.0.2.10:5555"
	rec := serve(t, h, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestAllowedClientPasses(t *testing.T) {
	upstream, _ := newUpstream(t, "admin ok")
	h, _ := newHandler(t, []config.Route{
		{Name: "admin", Prefix: "/admin", Upstream: upstream.URL, Allow: []string{"10.0.0.0/8"}},
	})
	req := httptest.NewRequest("GET", "http://hopfence.local/admin", nil)
	req.RemoteAddr = "10.1.2.3:5555"
	rec := serve(t, h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestDenyPathsBlocksSubtrees(t *testing.T) {
	upstream, _ := newUpstream(t, "x")
	h, _ := newHandler(t, []config.Route{
		{Name: "static", Prefix: "/static", Upstream: upstream.URL, DenyPaths: []string{"/static/private"}},
	})

	req := httptest.NewRequest("GET", "http://hopfence.local/static/private/key.pem", nil)
	if rec := serve(t, h, req); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}

	req = httptest.NewRequest("GET", "http://hopfence.local/static/app.js", nil)
	if rec := serve(t, h, req); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestUpstreamFailureReturnsBadGateway(t *testing.T) {
	h, _ := newHandler(t, []config.Route{
		{Name: "api", Prefix: "/api", Upstream: "http://127.0.0.1:1"},
	})
	rec := serve(t, h, httptest.NewRequest("GET", "http://hopfence.local/api", nil))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
}

func TestLogsStatusAndMethod(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer upstream.Close()

	h, logs := newHandler(t, []config.Route{
		{Name: "api", Prefix: "/api", Upstream: upstream.URL},
	})
	req := httptest.NewRequest("POST", "http://hopfence.local/api/items", nil)
	req.Header.Set("User-Agent", "integration-test/1.0")
	serve(t, h, req)

	line := strings.TrimSuffix(logs.String(), "\n")
	fields := strings.Split(line, " ")
	if len(fields) != 5 {
		t.Fatalf("log fields = %q", fields)
	}
	if fields[1] != "POST" {
		t.Errorf("method field = %q", fields[1])
	}
	if fields[3] != "201" {
		t.Errorf("status field = %q", fields[3])
	}
	if fields[4] != "integration-test/1.0" {
		t.Errorf("agent field = %q", fields[4])
	}
}
