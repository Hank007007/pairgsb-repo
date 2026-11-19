package store

import (
	"bytes"
	"errors"
	"testing"
)

// TestGetRejectsCrossTenantAccess reproduces the review finding: knowing
// another tenant's shard-id must not reveal the payload or its metadata.
func TestGetRejectsCrossTenantAccess(t *testing.T) {
	s := newTestStore(t)
	payload := []byte("alpha only payload")
	if err := s.Put("tenant-alpha", "alpha-1", "split-a", 1, 1, 3, payload); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, rec, err := s.Get("tenant-beta", "alpha-1")
	if !errors.Is(err, ErrCustodyMismatch) {
		t.Fatalf("cross-tenant Get: err = %v, want ErrCustodyMismatch", err)
	}
	if got != nil {
		t.Fatalf("cross-tenant Get leaked payload bytes: %x", got)
	}
	if rec.ShardID != "" || rec.CustodyID != "" || rec.Ciphertext != nil {
		t.Fatalf("cross-tenant Get leaked record metadata: %+v", rec)
	}

	// The owning tenant still has normal access.
	ownerPayload, _, err := s.Get("tenant-alpha", "alpha-1")
	if err != nil {
		t.Fatalf("owner Get: %v", err)
	}
	if !bytes.Equal(ownerPayload, payload) {
		t.Fatalf("owner Get payload = %x, want %x", ownerPayload, payload)
	}
}

func TestGetCrossTenantMissingAndDeniedDistinct(t *testing.T) {
	s := newTestStore(t)
	if err := s.Put("tenant-alpha", "alpha-1", "split-a", 1, 1, 2, []byte("a")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if _, _, err := s.Get("tenant-beta", "alpha-1"); !errors.Is(err, ErrCustodyMismatch) {
		t.Fatalf("cross-tenant Get: err = %v, want ErrCustodyMismatch", err)
	}
	if _, _, err := s.Get("tenant-beta", "unknown-id"); !errors.Is(err, ErrShardNotFound) {
		t.Fatalf("missing Get: err = %v, want ErrShardNotFound", err)
	}
}
