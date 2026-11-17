package fetcher

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"example.com/outfetch/internal/policy"
)

type resolverFunc func(string) ([]net.IP, error)

func (f resolverFunc) LookupIP(host string) ([]net.IP, error) {
	return f(host)
}

func newLocalDialer(t *testing.T, addr string) func(context.Context, string, string) (net.Conn, error) {
	t.Helper()
	return func(ctx context.Context, network, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}
}

func prefixDigest(b []byte, n int64) string {
	if int64(len(b)) > n {
		b = b[:n]
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestGetBlocksRedirectToLoopback(t *testing.T) {
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://127.0.0.1:8088/healthz", http.StatusFound)
	}))
	defer redirector.Close()

	p := policy.New(nil, nil, []int{80, 443, 8088}, resolverFunc(func(string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}))
	f := New(p)
	f.dial = newLocalDialer(t, redirector.Listener.Addr().String())

	_, err := f.Get(context.Background(), "http://first.example/")
	if !errors.Is(err, policy.ErrBlockedIP) {
		t.Fatalf("redirect target must be rechecked and blocked, got %v", err)
	}
}

func TestGetRechecksEveryRedirectHop(t *testing.T) {
	var mu sync.Mutex
	lookups := 0
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, "done")
	}))
	defer redirector.Close()

	p := policy.New([]string{"first.example"}, nil, []int{80}, resolverFunc(func(string) ([]net.IP, error) {
		mu.Lock()
		lookups++
		mu.Unlock()
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}))
	f := New(p)
	f.dial = newLocalDialer(t, redirector.Listener.Addr().String())

	res, err := f.Get(context.Background(), "http://first.example/start")
	if err != nil {
		t.Fatalf("relative redirect should be followed: %v", err)
	}
	if res.Status != http.StatusOK {
		t.Fatalf("want 200, got %d", res.Status)
	}
	if got := prefixDigest([]byte("done"), 1<<20); res.Digest != got {
		t.Fatalf("unexpected digest: %s", res.Digest)
	}
	mu.Lock()
	defer mu.Unlock()
	if lookups != 2 {
		t.Fatalf("each hop must be resolved and checked, got %d lookups", lookups)
	}
}

func TestGetRejectsRedirectToDisallowedHost(t *testing.T) {
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://other.example/", http.StatusFound)
	}))
	defer redirector.Close()

	p := policy.New([]string{"first.example"}, nil, []int{80}, resolverFunc(func(string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}))
	f := New(p)
	f.dial = newLocalDialer(t, redirector.Listener.Addr().String())

	_, err := f.Get(context.Background(), "http://first.example/")
	if !errors.Is(err, policy.ErrHostDenied) {
		t.Fatalf("want ErrHostDenied on redirect, got %v", err)
	}
}

func TestGetRejectsRedirectToDisallowedPort(t *testing.T) {
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://first.example:2222/", http.StatusFound)
	}))
	defer redirector.Close()

	p := policy.New([]string{"first.example"}, nil, []int{80, 443}, resolverFunc(func(string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}))
	f := New(p)
	f.dial = newLocalDialer(t, redirector.Listener.Addr().String())

	_, err := f.Get(context.Background(), "http://first.example/")
	if !errors.Is(err, policy.ErrPortDenied) {
		t.Fatalf("want ErrPortDenied on redirect, got %v", err)
	}
}

func TestGetEnforcesMaxRedirects(t *testing.T) {
	var mu sync.Mutex
	requests := 0
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()
		http.Redirect(w, r, "/loop", http.StatusFound)
	}))
	defer redirector.Close()

	p := policy.New([]string{"first.example"}, nil, []int{80}, resolverFunc(func(string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}))
	f := New(p)
	f.MaxRedirects = 2
	f.dial = newLocalDialer(t, redirector.Listener.Addr().String())

	_, err := f.Get(context.Background(), "http://first.example/loop")
	if !errors.Is(err, ErrTooManyRedirects) {
		t.Fatalf("want ErrTooManyRedirects, got %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if requests != 3 {
		t.Fatalf("want initial request plus two redirects, got %d", requests)
	}
}

func TestGetPinsResolvedAddressAndDoesNotResolveAgain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "pinned")
	}))
	defer server.Close()

	var mu sync.Mutex
	lookups := 0
	p := policy.New([]string{"rebind.example"}, nil, []int{80}, resolverFunc(func(string) ([]net.IP, error) {
		mu.Lock()
		defer mu.Unlock()
		lookups++
		if lookups == 1 {
			return []net.IP{net.ParseIP("93.184.216.34")}, nil
		}
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	}))
	f := New(p)
	var dialed []string
	f.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		mu.Lock()
		dialed = append(dialed, address)
		mu.Unlock()
		var d net.Dialer
		return d.DialContext(ctx, network, server.Listener.Addr().String())
	}

	if _, err := f.Get(context.Background(), "http://rebind.example/"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if lookups != 1 {
		t.Fatalf("address must be resolved once per hop, got %d lookups", lookups)
	}
	if len(dialed) != 1 || !strings.HasPrefix(dialed[0], "93.184.216.34:") {
		t.Fatalf("dial must use the checked address, got %v", dialed)
	}
}

func TestGetPreservesTLSConfigAndServerName(t *testing.T) {
	var mu sync.Mutex
	var serverName string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		serverName = r.TLS.ServerName
		mu.Unlock()
		_, _ = io.WriteString(w, "secure")
	}))
	defer server.Close()

	p := policy.New([]string{"first.example"}, nil, []int{443}, resolverFunc(func(string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}))
	f := New(p)
	f.Client = &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	f.dial = newLocalDialer(t, server.Listener.Addr().String())

	res, err := f.Get(context.Background(), "https://first.example/")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if res.Status != http.StatusOK {
		t.Fatalf("want 200, got %d", res.Status)
	}
	mu.Lock()
	defer mu.Unlock()
	if serverName != "first.example" {
		t.Fatalf("TLS SNI should keep the original host, got %q", serverName)
	}
}

func TestGetRejectsPrivateHostnameBeforeDial(t *testing.T) {
	dialed := false
	p := policy.New(nil, nil, nil, resolverFunc(func(string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("10.1.2.3")}, nil
	}))
	f := New(p)
	f.dial = func(context.Context, string, string) (net.Conn, error) {
		dialed = true
		return nil, errors.New("unexpected dial")
	}

	_, err := f.Get(context.Background(), "http://metadata.internal/latest/meta-data/")
	if !errors.Is(err, policy.ErrBlockedIP) {
		t.Fatalf("want ErrBlockedIP, got %v", err)
	}
	if dialed {
		t.Fatal("blocked hostname must not be dialed")
	}
}

func TestGetRejectsMetadataLiteral(t *testing.T) {
	f := New(policy.New(nil, nil, []int{80, 443}, nil))
	_, err := f.Get(context.Background(), "http://169.254.169.254/latest/meta-data/")
	if !errors.Is(err, policy.ErrBlockedIP) {
		t.Fatalf("want ErrBlockedIP, got %v", err)
	}
}

func TestGetRejectsDisallowedPortBeforeDial(t *testing.T) {
	dialed := false
	p := policy.New([]string{"first.example"}, nil, []int{80, 443}, resolverFunc(func(string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}))
	f := New(p)
	f.dial = func(context.Context, string, string) (net.Conn, error) {
		dialed = true
		return nil, errors.New("unexpected dial")
	}

	_, err := f.Get(context.Background(), "http://first.example:2222/")
	if !errors.Is(err, policy.ErrPortDenied) {
		t.Fatalf("want ErrPortDenied, got %v", err)
	}
	if dialed {
		t.Fatal("disallowed port must not be dialed")
	}
}

func TestGetTruncatesLargeBody(t *testing.T) {
	body := bytes.Repeat([]byte("0123456789"), 100000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()

	f := New(nil)
	f.MaxBytes = 1024
	res, err := f.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if res.Bytes != 1024 || !res.Truncated {
		t.Fatalf("want 1024 truncated bytes, got bytes=%d truncated=%v", res.Bytes, res.Truncated)
	}
	if want := prefixDigest(body, 1024); res.Digest != want {
		t.Fatalf("digest should cover only the bytes read: want %s got %s", want, res.Digest)
	}
}

type guardedReader struct {
	data     []byte
	off      int
	maxReads int
}

func (r *guardedReader) Read(p []byte) (int, error) {
	if r.off >= r.maxReads {
		return 0, errors.New("reader consumed more than the configured limit")
	}
	if remaining := r.maxReads - r.off; len(p) > remaining {
		p = p[:remaining]
	}
	if r.off >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.off:])
	r.off += n
	return n, nil
}

func TestReadBodyDoesNotConsumeWholeResponse(t *testing.T) {
	body := bytes.Repeat([]byte("a"), 1<<20)
	r := &guardedReader{data: body, maxReads: 1025}
	n, digest, truncated, err := readBody(r, 1024)
	if err != nil {
		t.Fatalf("readBody: %v", err)
	}
	if n != 1024 || !truncated {
		t.Fatalf("want 1024 truncated bytes, got n=%d truncated=%v", n, truncated)
	}
	if want := prefixDigest(body, 1024); digest != want {
		t.Fatalf("unexpected digest: %s", digest)
	}
	if r.off != 1025 {
		t.Fatalf("reader consumed %d bytes, want at most 1025", r.off)
	}
}
