package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kekring/internal/keyring"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	kr := keyring.New([]keyring.Key{{
		Version:  1,
		Status:   keyring.StatusActive,
		Material: bytes.Repeat([]byte{0x3c}, keyring.KeySize),
	}})
	srv := httptest.NewServer(New(kr).Handler())
	t.Cleanup(srv.Close)
	return srv
}

func doRequest(t *testing.T, method, url string, body []byte) (*http.Response, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return resp, payload
}

func TestHealth(t *testing.T) {
	srv := newTestServer(t)

	resp, body := doRequest(t, http.MethodGet, srv.URL+"/healthz", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
}

func TestPutThenGetObject(t *testing.T) {
	srv := newTestServer(t)
	plaintext := []byte("hello from the object store")

	resp, body := doRequest(t, http.MethodPut, srv.URL+"/objects/docs%2Freadme.txt", plaintext)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("PUT status = %d, want 201 (body %s)", resp.StatusCode, body)
	}

	var created objectResponse
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode PUT response: %v", err)
	}
	if created.ID != "docs/readme.txt" {
		t.Errorf("id = %q, want docs/readme.txt", created.ID)
	}
	if created.Size != len(plaintext) {
		t.Errorf("size = %d, want %d", created.Size, len(plaintext))
	}

	resp, body = doRequest(t, http.MethodGet, srv.URL+"/objects/docs%2Freadme.txt", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	if !bytes.Equal(body, plaintext) {
		t.Fatal("GET returned different bytes than were stored")
	}
}

func TestGetMissingObject(t *testing.T) {
	srv := newTestServer(t)

	resp, _ := doRequest(t, http.MethodGet, srv.URL+"/objects/nope", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestRewrapKeepsObjectReadable(t *testing.T) {
	srv := newTestServer(t)
	plaintext := []byte("rewrap must not touch the body")

	resp, body := doRequest(t, http.MethodPut, srv.URL+"/objects/a", plaintext)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("PUT status = %d, want 201 (body %s)", resp.StatusCode, body)
	}

	rotateBody, err := json.Marshal(rotateRequest{
		KEK: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x4d}, keyring.KeySize)),
	})
	if err != nil {
		t.Fatalf("marshal rotate request: %v", err)
	}
	resp, body = doRequest(t, http.MethodPost, srv.URL+"/keyring/rotate", rotateBody)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rotate status = %d, want 200 (body %s)", resp.StatusCode, body)
	}

	resp, body = doRequest(t, http.MethodPost, srv.URL+"/objects/a/rewrap", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rewrap status = %d, want 200 (body %s)", resp.StatusCode, body)
	}

	resp, body = doRequest(t, http.MethodGet, srv.URL+"/objects/a", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	if !bytes.Equal(body, plaintext) {
		t.Fatal("object changed after rewrap")
	}
}

func TestListKeyring(t *testing.T) {
	srv := newTestServer(t)

	resp, body := doRequest(t, http.MethodGet, srv.URL+"/keyring", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", resp.StatusCode, body)
	}

	var listing keyringResponse
	if err := json.Unmarshal(body, &listing); err != nil {
		t.Fatalf("decode listing: %v", err)
	}
	if len(listing.Keys) != 1 {
		t.Fatalf("listed %d keys, want 1", len(listing.Keys))
	}
	if listing.Keys[0].Version != 1 || listing.Keys[0].Status != keyring.StatusActive {
		t.Errorf("listed key = %+v, want version 1 active", listing.Keys[0])
	}
}

func TestRotateRejectsBadBase64(t *testing.T) {
	srv := newTestServer(t)

	resp, _ := doRequest(t, http.MethodPost, srv.URL+"/keyring/rotate", []byte(`{"kek":"not base64!"}`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}

	resp, _ = doRequest(t, http.MethodPost, srv.URL+"/keyring/rotate", []byte(strings.Repeat("x", 10)))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed JSON status = %d, want 400", resp.StatusCode)
	}
}
