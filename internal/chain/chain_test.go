package chain

import (
	"errors"
	"testing"
	"time"

	"capbaton/internal/mac"
	"capbaton/internal/token"
)

var (
	testNow = time.Date(2029, 6, 1, 12, 0, 0, 0, time.UTC)
	testExp = "2029-12-31T23:59:59Z"

	testKeys = mac.Keyring{
		"alice": mac.Key("alice-secret"),
		"bob":   mac.Key("bob-secret"),
		"carol": mac.Key("carol-secret"),
	}
)

func testOptions(revoked RevocationList) Options {
	return Options{
		Keys:    testKeys,
		Revoked: revoked,
		Now:     func() time.Time { return testNow },
	}
}

func rootLink(t *testing.T) token.Link {
	t.Helper()
	link, err := IssueRoot(Claim{
		TokenID: "tok-alice-root",
		Issuer:  "alice",
		Subject: "bob",
		Scopes:  []string{"s3:read:bucket/incident-42", "s3:list:bucket/incident-42"},
		Caveats: []token.Caveat{{Name: CaveatExpires, Value: testExp}},
	}, testKeys["alice"])
	if err != nil {
		t.Fatalf("IssueRoot: %v", err)
	}
	return link
}

func childLink(t *testing.T, parent token.Link) token.Link {
	t.Helper()
	link, err := Narrow(parent, Claim{
		TokenID: "tok-bob-child",
		Issuer:  "bob",
		Subject: "carol",
		Scopes:  []string{"s3:read:bucket/incident-42/logs"},
		Caveats: append([]token.Caveat(nil), parent.Payload.Caveats...),
	}, testKeys["bob"])
	if err != nil {
		t.Fatalf("Narrow: %v", err)
	}
	return link
}

func TestIssueRootSignsPayload(t *testing.T) {
	root := rootLink(t)
	if root.Payload.PrevMAC != "" {
		t.Fatalf("root PrevMAC = %q, want empty", root.Payload.PrevMAC)
	}
	if root.MAC == "" {
		t.Fatal("root MAC is empty")
	}
	if err := mac.Verify(testKeys["alice"], root); err != nil {
		t.Fatalf("root MAC does not verify: %v", err)
	}
}

func TestVerifyChainSingleLink(t *testing.T) {
	root := rootLink(t)
	if err := VerifyChain([]token.Link{root}, testOptions(nil)); err != nil {
		t.Fatalf("VerifyChain: %v", err)
	}
}

func TestNarrowLinksParentMAC(t *testing.T) {
	root := rootLink(t)
	child := childLink(t, root)
	if child.Payload.PrevMAC != root.MAC {
		t.Fatalf("child PrevMAC = %q, want parent MAC %q", child.Payload.PrevMAC, root.MAC)
	}
	if err := mac.Verify(testKeys["bob"], child); err != nil {
		t.Fatalf("child MAC does not verify: %v", err)
	}
}

func TestVerifyChainTwoLinks(t *testing.T) {
	root := rootLink(t)
	child := childLink(t, root)
	if err := VerifyChain([]token.Link{root, child}, testOptions(nil)); err != nil {
		t.Fatalf("VerifyChain: %v", err)
	}
}

func TestNarrowRejectsNonDelegateIssuer(t *testing.T) {
	root := rootLink(t)
	_, err := Narrow(root, Claim{
		TokenID: "tok-mallory",
		Issuer:  "mallory",
		Subject: "dave",
		Scopes:  []string{"s3:read:bucket/incident-42/logs"},
	}, mac.Key("mallory-secret"))
	if !errors.Is(err, ErrIssuerNotDelegate) {
		t.Fatalf("Narrow error = %v, want ErrIssuerNotDelegate", err)
	}
}

func TestNarrowRejectsScopeOutsideParent(t *testing.T) {
	root := rootLink(t)
	_, err := Narrow(root, Claim{
		TokenID: "tok-bob-wide",
		Issuer:  "bob",
		Subject: "carol",
		Scopes:  []string{"s3:write:bucket/incident-42"},
	}, testKeys["bob"])
	if !errors.Is(err, ErrScopeNotSubset) {
		t.Fatalf("Narrow error = %v, want ErrScopeNotSubset", err)
	}
}

func TestNarrowRejectsUnchangedScope(t *testing.T) {
	root := rootLink(t)
	_, err := Narrow(root, Claim{
		TokenID: "tok-bob-same",
		Issuer:  "bob",
		Subject: "carol",
		Scopes:  append([]string(nil), root.Payload.Scopes...),
	}, testKeys["bob"])
	if !errors.Is(err, ErrScopeNotNarrowed) {
		t.Fatalf("Narrow error = %v, want ErrScopeNotNarrowed", err)
	}
}

func TestNarrowRequiresIdentityFields(t *testing.T) {
	root := rootLink(t)
	_, err := Narrow(root, Claim{
		Issuer:  "bob",
		Subject: "carol",
		Scopes:  []string{"s3:read:bucket/incident-42/logs"},
	}, testKeys["bob"])
	if !errors.Is(err, ErrInvalidClaim) {
		t.Fatalf("Narrow error = %v, want ErrInvalidClaim", err)
	}
}

func TestVerifyChainRejectsEmptyChain(t *testing.T) {
	if err := VerifyChain(nil, testOptions(nil)); !errors.Is(err, ErrEmptyChain) {
		t.Fatalf("VerifyChain error = %v, want ErrEmptyChain", err)
	}
}

func TestVerifyChainRejectsUnknownIssuer(t *testing.T) {
	root := rootLink(t)
	opts := testOptions(nil)
	opts.Keys = mac.Keyring{"bob": testKeys["bob"]}
	if err := VerifyChain([]token.Link{root}, opts); !errors.Is(err, ErrUnknownIssuer) {
		t.Fatalf("VerifyChain error = %v, want ErrUnknownIssuer", err)
	}
}

func TestVerifyChainRejectsTamperedLink(t *testing.T) {
	root := rootLink(t)
	child := childLink(t, root)
	child.Payload.Subject = "mallory"
	if err := VerifyChain([]token.Link{root, child}, testOptions(nil)); !errors.Is(err, ErrBrokenLink) {
		t.Fatalf("VerifyChain error = %v, want ErrBrokenLink", err)
	}
}

func TestVerifyChainRejectsRevokedLeaf(t *testing.T) {
	root := rootLink(t)
	child := childLink(t, root)
	opts := testOptions(staticRevocation{"tok-bob-child": true})
	if err := VerifyChain([]token.Link{root, child}, opts); !errors.Is(err, ErrRevoked) {
		t.Fatalf("VerifyChain error = %v, want ErrRevoked", err)
	}
}

// staticRevocation is a RevocationList backed by a fixed set of token IDs.
type staticRevocation map[string]bool

func (s staticRevocation) IsRevoked(links []token.Link) bool {
	if len(links) == 0 {
		return false
	}
	return s[links[len(links)-1].Payload.TokenID]
}
