package api

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/outfetch/internal/fetcher"
	"example.com/outfetch/internal/policy"
)

type staticResolver map[string][]net.IP

func (r staticResolver) LookupIP(host string) ([]net.IP, error) {
	if ips, ok := r[host]; ok {
		return ips, nil
	}
	return []net.IP{net.ParseIP("93.184.216.34")}, nil
}

func newSecurityServer(t *testing.T, p *policy.Policy) *httptest.Server {
	t.Helper()
	return httptest.NewServer(New(p, fetcher.New(p)).Routes())
}

func postFetch(t *testing.T, baseURL, target string) (*http.Response, fetchResponse) {
	t.Helper()
	resp, err := http.Post(baseURL+"/fetch", "application/json", strings.NewReader(`{"url":"`+target+`"}`))
	if err != nil {
		t.Fatalf("post fetch: %v", err)
	}
	var body fetchResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		resp.Body.Close()
		t.Fatalf("decode: %v", err)
	}
	resp.Body.Close()
	return resp, body
}

func TestFetchDeniesAllowlistSuffixLookalike(t *testing.T) {
	p := policy.New([]string{"shop.example.com"}, nil, []int{80, 443}, staticResolver{})
	ts := newSecurityServer(t, p)
	defer ts.Close()

	resp, body := postFetch(t, ts.URL, "http://notshop.example.com/")
	if resp.StatusCode != http.StatusForbidden || body.Error != ErrPolicyDenied {
		t.Fatalf("want 403 policy_denied, got %d %+v", resp.StatusCode, body)
	}
	if body.Detail == "" {
		t.Fatal("policy denial should carry a detail")
	}
}

func TestFetchDeniesMetadataAddress(t *testing.T) {
	p := policy.New(nil, nil, []int{80, 443}, staticResolver{})
	ts := newSecurityServer(t, p)
	defer ts.Close()

	resp, body := postFetch(t, ts.URL, "http://169.254.169.254/latest/meta-data/")
	if resp.StatusCode != http.StatusForbidden || body.Error != ErrPolicyDenied {
		t.Fatalf("want 403 policy_denied, got %d %+v", resp.StatusCode, body)
	}
	if strings.Contains(body.Detail, "169.254.169.254") {
		t.Fatalf("detail must not echo the blocked address: %q", body.Detail)
	}
}

func TestFetchDeniesPrivateHostnameWithoutAllowlist(t *testing.T) {
	p := policy.New(nil, nil, []int{80, 443}, staticResolver{
		"metadata.internal": {net.ParseIP("10.12.34.56")},
	})
	ts := newSecurityServer(t, p)
	defer ts.Close()

	resp, body := postFetch(t, ts.URL, "http://metadata.internal/latest/meta-data/")
	if resp.StatusCode != http.StatusForbidden || body.Error != ErrPolicyDenied {
		t.Fatalf("want 403 policy_denied, got %d %+v", resp.StatusCode, body)
	}
	if strings.Contains(body.Detail, "10.12.34.56") {
		t.Fatalf("detail must not echo the resolved private address: %q", body.Detail)
	}
}

func TestFetchDeniesLoopbackLiteral(t *testing.T) {
	p := policy.New(nil, nil, []int{80, 443}, staticResolver{})
	ts := newSecurityServer(t, p)
	defer ts.Close()

	resp, body := postFetch(t, ts.URL, "http://127.0.0.1:8088/healthz")
	if resp.StatusCode != http.StatusForbidden || body.Error != ErrPolicyDenied {
		t.Fatalf("want 403 policy_denied, got %d %+v", resp.StatusCode, body)
	}
}

func TestFetchDeniesPortOutsideAllowlist(t *testing.T) {
	p := policy.New([]string{"shop.example.com"}, nil, []int{80, 443}, staticResolver{})
	ts := newSecurityServer(t, p)
	defer ts.Close()

	resp, body := postFetch(t, ts.URL, "http://shop.example.com:2222/")
	if resp.StatusCode != http.StatusForbidden || body.Error != ErrPolicyDenied {
		t.Fatalf("want 403 policy_denied, got %d %+v", resp.StatusCode, body)
	}
}
