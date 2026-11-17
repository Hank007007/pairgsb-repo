package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"kekring/internal/keyring"
)

func TestPutAfterRotationUsesActiveVersion(t *testing.T) {
	kr := keyring.New([]keyring.Key{{
		Version:  1,
		Status:   keyring.StatusActive,
		Material: bytes.Repeat([]byte{0x3c}, keyring.KeySize),
	}})
	srv := httptest.NewServer(New(kr).Handler())
	t.Cleanup(srv.Close)

	resp, body := doRequest(t, http.MethodPut, srv.URL+"/objects/before", []byte("before rotation"))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("PUT before rotation status = %d, want 201 (body %s)", resp.StatusCode, body)
	}
	var before objectResponse
	if err := json.Unmarshal(body, &before); err != nil {
		t.Fatalf("decode PUT response: %v", err)
	}
	if before.KEKVersion != 1 {
		t.Fatalf("before rotation KEKVersion = %d, want 1", before.KEKVersion)
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

	resp, body = doRequest(t, http.MethodGet, srv.URL+"/keyring", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET keyring status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	var listing keyringResponse
	if err := json.Unmarshal(body, &listing); err != nil {
		t.Fatalf("decode keyring listing: %v", err)
	}
	if len(listing.Keys) != 2 {
		t.Fatalf("listed %d keys, want 2", len(listing.Keys))
	}
	if listing.Keys[0].Version != 1 || listing.Keys[0].Status != keyring.StatusRetired {
		t.Errorf("version 1 = %+v, want retired", listing.Keys[0])
	}
	if listing.Keys[1].Version != 2 || listing.Keys[1].Status != keyring.StatusActive {
		t.Errorf("version 2 = %+v, want active", listing.Keys[1])
	}

	resp, body = doRequest(t, http.MethodPut, srv.URL+"/objects/after", []byte("after rotation"))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("PUT after rotation status = %d, want 201 (body %s)", resp.StatusCode, body)
	}
	var after objectResponse
	if err := json.Unmarshal(body, &after); err != nil {
		t.Fatalf("decode PUT response: %v", err)
	}
	if after.KEKVersion != 2 {
		t.Fatalf("after rotation KEKVersion = %d, want 2", after.KEKVersion)
	}

	resp, body = doRequest(t, http.MethodGet, srv.URL+"/objects/before", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET pre-rotation object status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	if !bytes.Equal(body, []byte("before rotation")) {
		t.Fatal("pre-rotation object changed after rotation")
	}
}

func TestPutFailsWithoutActiveKey(t *testing.T) {
	kr := keyring.New([]keyring.Key{{
		Version:  1,
		Status:   keyring.StatusRetired,
		Material: bytes.Repeat([]byte{0x3c}, keyring.KeySize),
	}})
	srv := httptest.NewServer(New(kr).Handler())
	t.Cleanup(srv.Close)

	resp, body := doRequest(t, http.MethodPut, srv.URL+"/objects/a", []byte("payload"))
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("PUT status = %d, want 500 (body %s)", resp.StatusCode, body)
	}
}

func TestRotateRejectsInvalidKEKLength(t *testing.T) {
	srv := newTestServer(t)

	rotateBody, err := json.Marshal(rotateRequest{
		KEK: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x4d}, 20)),
	})
	if err != nil {
		t.Fatalf("marshal rotate request: %v", err)
	}
	resp, body := doRequest(t, http.MethodPost, srv.URL+"/keyring/rotate", rotateBody)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("rotate status = %d, want 400 (body %s)", resp.StatusCode, body)
	}
}
