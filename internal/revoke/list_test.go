package revoke

import (
	"testing"

	"capbaton/internal/token"
)

func TestRevokeAndHas(t *testing.T) {
	l := New()
	if l.Has("tok-1") {
		t.Fatal("fresh list reports a revocation")
	}
	l.Revoke("tok-1")
	if !l.Has("tok-1") {
		t.Fatal("Revoke did not record the token id")
	}
	if l.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", l.Len())
	}
}

func TestRevokeIsIdempotent(t *testing.T) {
	l := New()
	l.Revoke("tok-1")
	l.Revoke("tok-1")
	if l.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", l.Len())
	}
}

func TestRevokeIgnoresEmptyID(t *testing.T) {
	l := New()
	l.Revoke("")
	if l.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", l.Len())
	}
}

func TestIsRevokedSingleLink(t *testing.T) {
	l := New()
	links := []token.Link{{Payload: token.Payload{TokenID: "tok-1"}}}
	if l.IsRevoked(links) {
		t.Fatal("list reports an unrevoked chain as revoked")
	}
	l.Revoke("tok-1")
	if !l.IsRevoked(links) {
		t.Fatal("list failed to report a revoked single-link chain")
	}
}

func TestIsRevokedEmptyChain(t *testing.T) {
	if New().IsRevoked(nil) {
		t.Fatal("empty chain reported as revoked")
	}
}

func TestNilListIsSafe(t *testing.T) {
	var l *List
	if l.IsRevoked([]token.Link{{Payload: token.Payload{TokenID: "tok-1"}}}) {
		t.Fatal("nil list reported a revocation")
	}
}
