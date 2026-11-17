package mac

import (
	"testing"
	"time"

	"capbaton/internal/token"
)

func samplePayload() token.Payload {
	return token.Payload{
		TokenID:  "tok-root",
		Issuer:   "alice",
		Subject:  "bob",
		Scopes:   []string{"s3:read:bucket/incident-42"},
		Caveats:  []token.Caveat{{Name: "expires", Value: "2030-01-01T00:00:00Z"}},
		IssuedAt: time.Date(2029, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestComputeIsStable(t *testing.T) {
	key := Key("k1")
	a := Compute(key, samplePayload())
	b := Compute(key, samplePayload())
	if a != b {
		t.Fatalf("Compute is not deterministic: %q vs %q", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("MAC length = %d, want 64 hex chars", len(a))
	}
}

func TestComputeCoversEveryField(t *testing.T) {
	key := Key("k1")
	base := Compute(key, samplePayload())

	mutations := map[string]func(*token.Payload){
		"token id":  func(p *token.Payload) { p.TokenID = "tok-other" },
		"issuer":    func(p *token.Payload) { p.Issuer = "mallory" },
		"subject":   func(p *token.Payload) { p.Subject = "carol" },
		"scopes":    func(p *token.Payload) { p.Scopes = []string{"s3:write:bucket/incident-42"} },
		"caveats":   func(p *token.Payload) { p.Caveats = nil },
		"prev mac":  func(p *token.Payload) { p.PrevMAC = "deadbeef" },
		"issued at": func(p *token.Payload) { p.IssuedAt = p.IssuedAt.Add(time.Second) },
	}
	for name, mutate := range mutations {
		p := samplePayload()
		mutate(&p)
		if got := Compute(key, p); got == base {
			t.Fatalf("changing %s did not change the MAC", name)
		}
	}
}

func TestVerifyAcceptsSignedLink(t *testing.T) {
	key := Key("k1")
	p := samplePayload()
	link := token.Link{Payload: p, MAC: Compute(key, p)}
	if err := Verify(key, link); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestVerifyRejectsTamperedPayload(t *testing.T) {
	key := Key("k1")
	p := samplePayload()
	link := token.Link{Payload: p, MAC: Compute(key, p)}
	link.Payload.Scopes = append(link.Payload.Scopes, "s3:delete:bucket/incident-42")
	if err := Verify(key, link); err == nil {
		t.Fatal("Verify accepted a tampered payload")
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	p := samplePayload()
	link := token.Link{Payload: p, MAC: Compute(Key("k1"), p)}
	if err := Verify(Key("k2"), link); err == nil {
		t.Fatal("Verify accepted a link signed with another key")
	}
}

func TestKeyringLookup(t *testing.T) {
	kr := Keyring{"alice": Key("k1")}
	if _, ok := kr.Lookup("alice"); !ok {
		t.Fatal("Lookup(alice) failed")
	}
	if _, ok := kr.Lookup("bob"); ok {
		t.Fatal("Lookup(bob) unexpectedly succeeded")
	}
}
