// Command probe reproduces the attack combinations raised in the
// security review against a fixed keyring and a fixed clock. Every
// scenario must surface a distinguishable error (errors.Is); a happy
// path is checked as well so the hardening cannot reject legitimate
// chains.
package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"capbaton/internal/chain"
	"capbaton/internal/mac"
	"capbaton/internal/revoke"
	"capbaton/internal/scope"
	"capbaton/internal/token"
)

var (
	fixedNow = time.Date(2026, time.November, 18, 12, 0, 0, 0, time.UTC)
	keys     = mac.Keyring{
		"alice": mac.Key("alice-secret"),
		"bob":   mac.Key("bob-secret"),
		"carol": mac.Key("carol-secret"),
	}
)

func options(revoked chain.RevocationList) chain.Options {
	return chain.Options{
		Keys:    keys,
		Revoked: revoked,
		Now:     func() time.Time { return fixedNow },
	}
}

func craft(issuerKey mac.Key, p token.Payload) token.Link {
	return token.Link{Payload: p, MAC: mac.Compute(issuerKey, p)}
}

func expires(value string) []token.Caveat {
	return []token.Caveat{{Name: chain.CaveatExpires, Value: value}}
}

var failures int

func check(name string, err error, target error) {
	switch {
	case target == nil && err == nil:
		fmt.Printf("PASS %s: accepted as expected\n", name)
	case target == nil && err != nil:
		failures++
		fmt.Printf("FAIL %s: unexpected error %v\n", name, err)
	case err == nil:
		failures++
		fmt.Printf("FAIL %s: expected %v, got nil\n", name, target)
	case !errors.Is(err, target):
		failures++
		fmt.Printf("FAIL %s: expected %v, got %v\n", name, target, err)
	default:
		fmt.Printf("PASS %s: %v\n", name, err)
	}
}

// buildThreeLinkChain issues root (alice->bob), middle (bob->carol) and
// leaf (carol->dave) under the given token-id prefix. Two chains built
// with different prefixes carry the same scopes but different MACs.
func buildThreeLinkChain(prefix string) (root, middle, leaf token.Link) {
	var err error
	root, err = chain.IssueRoot(chain.Claim{
		TokenID: prefix + "-root",
		Issuer:  "alice",
		Subject: "bob",
		Scopes:  []string{"s3:read:bucket/incident-42", "s3:list:bucket/incident-42"},
		Caveats: expires("2030-01-01T00:00:00Z"),
	}, keys["alice"])
	if err != nil {
		panic(err)
	}
	middle, err = chain.Narrow(root, chain.Claim{
		TokenID: prefix + "-middle",
		Issuer:  "bob",
		Subject: "carol",
		Scopes:  []string{"s3:read:bucket/incident-42/logs"},
		Caveats: append([]token.Caveat(nil), root.Payload.Caveats...),
	}, keys["bob"])
	if err != nil {
		panic(err)
	}
	leaf, err = chain.Narrow(middle, chain.Claim{
		TokenID: prefix + "-leaf",
		Issuer:  "carol",
		Subject: "dave",
		Scopes:  []string{"s3:read:bucket/incident-42/logs/app"},
		Caveats: append([]token.Caveat(nil), middle.Payload.Caveats...),
	}, keys["carol"])
	if err != nil {
		panic(err)
	}
	return root, middle, leaf
}

func main() {
	probeSplice()
	probeCaveatDrop()
	probeScopeWiden()
	probeMiddleExpiry()
	probeIntermediateRevocation()
	probeHappyPath()

	if failures > 0 {
		fmt.Printf("\n%d probe scenario(s) failed\n", failures)
		os.Exit(1)
	}
	fmt.Println("\nall probe scenarios behaved as specified")
}

// probeSplice grafts the middle and leaf of a second, independently
// issued chain behind the first chain's root. Every link still verifies
// on its own, but the parent-MAC linkage must reject the assembly.
func probeSplice() {
	rootA, middleA, leafA := buildThreeLinkChain("a")
	_, middleB, leafB := buildThreeLinkChain("b")

	for i, link := range []token.Link{rootA, middleB, leafB} {
		key, _ := keys.Lookup(link.Payload.Issuer)
		if err := mac.Verify(key, link); err != nil {
			failures++
			fmt.Printf("FAIL splice: link %d no longer verifies on its own: %v\n", i, err)
			return
		}
	}
	check("splice VerifyChain", chain.VerifyChain(
		[]token.Link{rootA, middleB, leafB}, options(nil)), chain.ErrBrokenLink)

	// The untouched originals must remain intact.
	check("original chain A", chain.VerifyChain(
		[]token.Link{rootA, middleA, leafA}, options(nil)), nil)
}

// probeCaveatDrop tries to shed the root's expires caveat by passing no
// caveats when attenuating.
func probeCaveatDrop() {
	root, _, _ := buildThreeLinkChain("caveat")
	_, err := chain.Narrow(root, chain.Claim{
		TokenID: "caveat-drop-child",
		Issuer:  "bob",
		Subject: "carol",
		Scopes:  []string{"s3:read:bucket/incident-42/logs"},
		Caveats: nil,
	}, keys["bob"])
	check("caveat drop on Narrow", err, chain.ErrParentCaveatMissed)

	_, err = chain.Narrow(root, chain.Claim{
		TokenID: "caveat-change-child",
		Issuer:  "bob",
		Subject: "carol",
		Scopes:  []string{"s3:read:bucket/incident-42/logs"},
		Caveats: expires("2099-01-01T00:00:00Z"),
	}, keys["bob"])
	check("caveat value change on Narrow", err, chain.ErrParentCaveatMissed)
}

// probeScopeWiden exercises the sibling-directory confusion at both the
// scope package and the attenuation boundary.
func probeScopeWiden() {
	parent, err := scope.Parse("s3:read:bucket/ab")
	if err != nil {
		panic(err)
	}
	sibling, err := scope.Parse("s3:read:bucket/abc")
	if err != nil {
		panic(err)
	}
	if scope.IsSubset(sibling, parent) {
		failures++
		fmt.Println("FAIL scope siblings: bucket/abc reported as contained in bucket/ab")
	} else {
		fmt.Println("PASS scope siblings: bucket/abc is not contained in bucket/ab")
	}

	root, _, _ := buildThreeLinkChain("widen")
	_, err = chain.Narrow(root, chain.Claim{
		TokenID: "widen-child",
		Issuer:  "bob",
		Subject: "carol",
		Scopes:  []string{"s3:read:bucket/incident-421"},
		Caveats: append([]token.Caveat(nil), root.Payload.Caveats...),
	}, keys["bob"])
	check("scope widen on Narrow", err, chain.ErrScopeNotSubset)
}

// probeMiddleExpiry hand-builds a structurally valid chain whose middle
// link expired in 2020 while root and leaf claim 2030. The 2026 clock
// must reject the chain because of the intermediate link.
func probeMiddleExpiry() {
	root := craft(keys["alice"], token.Payload{
		TokenID:  "exp-root",
		Issuer:   "alice",
		Subject:  "bob",
		Scopes:   []string{"s3:read:bucket/incident-42"},
		Caveats:  expires("2030-01-01T00:00:00Z"),
		IssuedAt: fixedNow,
	})
	middle := craft(keys["bob"], token.Payload{
		TokenID:  "exp-middle",
		Issuer:   "bob",
		Subject:  "carol",
		Scopes:   []string{"s3:read:bucket/incident-42/logs"},
		Caveats:  expires("2020-01-01T00:00:00Z"),
		PrevMAC:  root.MAC,
		IssuedAt: fixedNow,
	})
	leaf := craft(keys["carol"], token.Payload{
		TokenID:  "exp-leaf",
		Issuer:   "carol",
		Subject:  "dave",
		Scopes:   []string{"s3:read:bucket/incident-42/logs/app"},
		Caveats:  expires("2030-01-01T00:00:00Z"),
		PrevMAC:  middle.MAC,
		IssuedAt: fixedNow,
	})
	check("middle link expired", chain.VerifyChain(
		[]token.Link{root, middle, leaf}, options(nil)), chain.ErrExpired)
}

// probeIntermediateRevocation revokes the middle link's token id. The
// leaf itself is not revoked, yet the whole chain must be rejected; the
// leaf-only case is checked as a control.
func probeIntermediateRevocation() {
	root, middle, leaf := buildThreeLinkChain("revoke")

	intermediateOnly := revoke.New()
	intermediateOnly.Revoke(middle.Payload.TokenID)
	check("intermediate revoked", chain.VerifyChain(
		[]token.Link{root, middle, leaf}, options(intermediateOnly)), chain.ErrRevoked)

	leafOnly := revoke.New()
	leafOnly.Revoke(leaf.Payload.TokenID)
	check("leaf revoked", chain.VerifyChain(
		[]token.Link{root, middle, leaf}, options(leafOnly)), chain.ErrRevoked)
}

// probeHappyPath confirms an intact, in-date, unrevoked chain still
// verifies after the hardening.
func probeHappyPath() {
	root, middle, leaf := buildThreeLinkChain("happy")
	check("happy path", chain.VerifyChain(
		[]token.Link{root, middle, leaf}, options(nil)), nil)
}
