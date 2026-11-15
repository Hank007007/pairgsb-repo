package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/outfetch/internal/fetcher"
	"example.com/outfetch/internal/policy"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	p := policy.New([]string{"shop.example.com"}, nil, []int{80, 443}, nil)
	s := New(p, fetcher.New(p))
	return httptest.NewServer(s.Routes())
}

func TestHealthz(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("get healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestFetchRequiresPost(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/fetch")
	if err != nil {
		t.Fatalf("get fetch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", resp.StatusCode)
	}
}

func TestFetchRejectsBadJSON(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/fetch", "application/json", strings.NewReader("{not json"))
	if err != nil {
		t.Fatalf("post fetch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}

	var body fetchResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.OK || body.Error != ErrBadRequest {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestFetchRejectsMissingURL(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/fetch", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("post fetch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}
