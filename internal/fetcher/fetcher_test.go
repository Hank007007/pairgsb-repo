package fetcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetReturnsDigest(t *testing.T) {
	body := []byte("hello egress")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write(body)
	}))
	defer srv.Close()

	// Policy 传 nil：这条用例只验抓取本身，准入规则由 policy 包的用例负责。
	f := New(nil)
	res, err := f.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	sum := sha256.Sum256(body)
	if res.Status != http.StatusTeapot {
		t.Errorf("status: want 418, got %d", res.Status)
	}
	if res.Bytes != len(body) {
		t.Errorf("bytes: want %d, got %d", len(body), res.Bytes)
	}
	if want := hex.EncodeToString(sum[:]); res.Digest != want {
		t.Errorf("digest: want %s, got %s", want, res.Digest)
	}
}

func TestGetRejectsBadURL(t *testing.T) {
	f := New(nil)
	if _, err := f.Get(context.Background(), "://no-scheme"); err == nil {
		t.Fatal("非法 URL 应当报错")
	}
}
