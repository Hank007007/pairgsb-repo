package test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"testing"
	"time"
)

// The fixed client key used across the suite. Its RFC 7638 thumbprint is
// du84AABnUEGGscLubYAY3-suCT--NFWTyPNH_UjdQNU.
const (
	clientX   = "7AyDoKI37Rc_cDd45aDLz8zWT8L4idKKQvP6i1JEmz4"
	clientY   = "Zn12zhye8mzE-nxD1TASDDR14D7rlNS6TI9TxNugHMw"
	clientD   = "3ueEU82b43W6leg9pvHJBe6DOmUN3fVWYzpXkaj_VQM"
	clientJKT = "Du84AABnUEGGscLubYAY3-suCT--NFWTyPNH_UjdQNU"
)

func clientKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	d, err := base64.RawURLEncoding.DecodeString(clientD)
	if err != nil {
		t.Fatalf("decode d: %v", err)
	}
	x, err := base64.RawURLEncoding.DecodeString(clientX)
	if err != nil {
		t.Fatalf("decode x: %v", err)
	}
	y, err := base64.RawURLEncoding.DecodeString(clientY)
	if err != nil {
		t.Fatalf("decode y: %v", err)
	}
	curve := elliptic.P256()
	return &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{Curve: curve, X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)},
		D:         new(big.Int).SetBytes(d),
	}
}

type proofOptions struct {
	Method      string
	URL         string
	AccessToken string
	JTI         string
	IAT         int64
	EXP         int64
	ExtraJWK    map[string]any
}

func signProof(t *testing.T, opts proofOptions) string {
	t.Helper()

	jwk := map[string]any{
		"kty": "EC",
		"crv": "P-256",
		"x":   clientX,
		"y":   clientY,
	}
	for k, v := range opts.ExtraJWK {
		jwk[k] = v
	}

	header := map[string]any{
		"typ": "dpop+jwt",
		"alg": "ES256",
		"jwk": jwk,
	}
	tokenSum := sha256.Sum256([]byte(opts.AccessToken))
	payload := map[string]any{
		"jti": opts.JTI,
		"htm": opts.Method,
		"htu": opts.URL,
		"iat": opts.IAT,
		"exp": opts.EXP,
		"ath": base64.RawURLEncoding.EncodeToString(tokenSum[:]),
	}

	return compactSign(t, header, payload)
}

func compactSign(t *testing.T, header, payload map[string]any) string {
	t.Helper()

	h, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	p, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(p)
	digest := sha256.Sum256([]byte(signingInput))

	sig, err := ecdsa.SignASN1(rand.Reader, clientKey(t), digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func newJTI(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func unixNow() int64 { return time.Now().Unix() }
