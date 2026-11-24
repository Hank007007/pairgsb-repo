package edge

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rangecarve/internal/objcache"
)

// fakeStore is an in-process stand-in for the object store. It holds one
// version per tenant and path and answers conditional requests the way the
// real store does.
type fakeStore struct {
	tenant  string
	path    string
	content []byte
	etag    string
	modTime time.Time
	calls   int
}

func (f *fakeStore) Get(ctx context.Context, tenant, objectPath string, hdr http.Header) (*http.Response, error) {
	f.calls++
	if tenant != f.tenant || objectPath != f.path {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     make(http.Header),
			Body:       http.NoBody,
		}, nil
	}

	if match := hdr.Get("If-None-Match"); match != "" && match == f.etag {
		header := make(http.Header)
		header.Set("ETag", f.etag)
		header.Set("Last-Modified", f.modTime.UTC().Format(http.TimeFormat))
		return &http.Response{
			StatusCode: http.StatusNotModified,
			Header:     header,
			Body:       http.NoBody,
		}, nil
	}

	header := make(http.Header)
	header.Set("ETag", f.etag)
	header.Set("Content-Type", "application/octet-stream")
	header.Set("Last-Modified", f.modTime.UTC().Format(http.TimeFormat))
	header.Set("X-Rangecarve-Filename", "clip.bin")
	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        header,
		Body:          newBody(f.content),
		ContentLength: int64(len(f.content)),
	}, nil
}

type sliceBody struct {
	*bytes.Reader
}

func newBody(b []byte) *sliceBody {
	return &sliceBody{bytes.NewReader(b)}
}

func (b *sliceBody) Close() error { return nil }

func newTestServer(t *testing.T) (*Server, *fakeStore) {
	t.Helper()
	content := make([]byte, 1000)
	for i := range content {
		content[i] = byte('a' + i%26)
	}
	store := &fakeStore{
		tenant:  "acme",
		path:    "/media/clip.bin",
		content: content,
		etag:    `"v3"`,
		modTime: time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC),
	}
	server := New(store, Config{Cache: objcache.New(16, time.Minute)})
	return server, store
}

func doGet(t *testing.T, server *Server, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("X-Rangecarve-Tenant", "acme")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	return rec
}

func TestGetWithoutRangeReturnsWholeObject(t *testing.T) {
	server, store := newTestServer(t)

	rec := doGet(t, server, "/media/clip.bin", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Body.Len(), len(store.content); got != want {
		t.Fatalf("body length = %d, want %d", got, want)
	}
	if !bytes.Equal(rec.Body.Bytes(), store.content) {
		t.Fatal("body does not match the stored object")
	}
	if got := rec.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestGetSingleByteRange(t *testing.T) {
	server, store := newTestServer(t)

	rec := doGet(t, server, "/media/clip.bin", map[string]string{
		"Range": "bytes=0-9",
	})

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusPartialContent)
	}
	if got, want := rec.Body.Len(), 10; got != want {
		t.Fatalf("body length = %d, want %d", got, want)
	}
	if !bytes.Equal(rec.Body.Bytes(), store.content[0:10]) {
		t.Fatal("body does not match the first ten bytes of the object")
	}
	if got := rec.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestGetWithMatchingETagIsNotModified(t *testing.T) {
	server, store := newTestServer(t)

	rec := doGet(t, server, "/media/clip.bin", map[string]string{
		"If-None-Match": store.etag,
	})

	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotModified)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body length = %d, want 0", rec.Body.Len())
	}
	if got := rec.Header().Get("ETag"); got != store.etag {
		t.Fatalf("ETag = %q, want %q", got, store.etag)
	}
}
