// Package proof parses and validates DPoP proof JWTs.
package proof

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"time"

	"proofbind/internal/jwk"
)

var (
	// ErrMalformed is returned when the proof is not a compact JWS with
	// decodable header and payload segments.
	ErrMalformed = errors.New("proof: malformed")
	// ErrUnsupportedType is returned when the JOSE typ is not dpop+jwt.
	ErrUnsupportedType = errors.New("proof: unexpected typ")
	// ErrUnsupportedAlgorithm is returned for algorithms other than ES256 and RS256.
	ErrUnsupportedAlgorithm = errors.New("proof: unsupported algorithm")
	// ErrSignature is returned when the signature does not verify.
	ErrSignature = errors.New("proof: bad signature")
	// ErrStale is returned when the proof is outside its validity window.
	ErrStale = errors.New("proof: outside validity window")
	// ErrMissingClaim is returned when a required claim is absent.
	ErrMissingClaim = errors.New("proof: missing required claim")
	// ErrMismatch is returned when the proof is not bound to the request
	// it arrived on (htm, htu or ath mismatch).
	ErrMismatch = errors.New("proof: not bound to this request")
)

// Request describes the request a proof was presented on.
type Request struct {
	Method      string
	Target      string
	AccessToken string
}

// Proof is a parsed proof whose signature has been verified.
type Proof struct {
	JTI    string
	HTM    string
	HTU    string
	ATH    string
	IAT    int64
	EXP    int64
	HasExp bool

	Key     *jwk.Key
	Alg     string
	Request Request
}

type header struct {
	Typ string          `json:"typ"`
	Alg string          `json:"alg"`
	JWK json.RawMessage `json:"jwk"`
}

type payload struct {
	JTI string `json:"jti"`
	HTM string `json:"htm"`
	HTU string `json:"htu"`
	ATH string `json:"ath"`
	IAT *int64 `json:"iat"`
	EXP *int64 `json:"exp"`
}

// Verify parses a compact proof JWS, resolves the public key from the header,
// and verifies the signature. The request the proof arrived on is retained
// alongside the claims.
func Verify(raw string, req Request) (*Proof, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, ErrMalformed
	}
	headJSON, err := decodeSegment(parts[0])
	if err != nil {
		return nil, ErrMalformed
	}
	payJSON, err := decodeSegment(parts[1])
	if err != nil {
		return nil, ErrMalformed
	}
	sig, err := decodeSegment(parts[2])
	if err != nil || len(sig) == 0 {
		return nil, ErrMalformed
	}

	var h header
	if err := json.Unmarshal(headJSON, &h); err != nil {
		return nil, ErrMalformed
	}
	if h.Typ != "dpop+jwt" {
		return nil, ErrUnsupportedType
	}
	if h.Alg != "ES256" && h.Alg != "RS256" {
		return nil, ErrUnsupportedAlgorithm
	}
	if len(h.JWK) == 0 {
		return nil, ErrMalformed
	}
	key, err := jwk.Parse(h.JWK)
	if err != nil {
		return nil, err
	}
	pub, err := key.PublicKey()
	if err != nil {
		return nil, err
	}

	signed := []byte(parts[0] + "." + parts[1])
	if err := checkSignature(h.Alg, pub, signed, sig); err != nil {
		return nil, err
	}

	var p payload
	if err := json.Unmarshal(payJSON, &p); err != nil {
		return nil, ErrMalformed
	}
	if p.JTI == "" || p.HTM == "" || p.HTU == "" || p.ATH == "" ||
		p.IAT == nil || p.EXP == nil {
		return nil, ErrMissingClaim
	}

	out := &Proof{
		JTI:     p.JTI,
		HTM:     p.HTM,
		HTU:     p.HTU,
		ATH:     p.ATH,
		IAT:     *p.IAT,
		EXP:     *p.EXP,
		HasExp:  true,
		Key:     key,
		Alg:     h.Alg,
		Request: req,
	}
	if err := out.bindRequest(); err != nil {
		return nil, err
	}
	return out, nil
}

// bindRequest checks the proof's htm, htu and ath claims against the request
// it was presented on.
func (p *Proof) bindRequest() error {
	if p.HTM != p.Request.Method {
		return ErrMismatch
	}
	if normalizeHTU(p.HTU) != normalizeHTU(p.Request.Target) {
		return ErrMismatch
	}
	if p.ATH != AccessTokenHash(p.Request.AccessToken) {
		return ErrMismatch
	}
	return nil
}

// normalizeHTU applies the normalization of docs/dpop-profile.md §4: scheme
// and host are lowercased, query and fragment are dropped, and a default
// port (:443 for https, :80 for http) is removed. The path is left exactly
// as received.
func normalizeHTU(raw string) string {
	if i := strings.Index(raw, "#"); i >= 0 {
		raw = raw[:i]
	}
	if i := strings.Index(raw, "?"); i >= 0 {
		raw = raw[:i]
	}
	scheme, rest, ok := strings.Cut(raw, "://")
	if !ok {
		return raw
	}
	scheme = strings.ToLower(scheme)
	host, path, hasPath := strings.Cut(rest, "/")
	host = strings.ToLower(host)
	if h, port, err := net.SplitHostPort(host); err == nil {
		if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
			host = h
		}
	}
	if !hasPath {
		return scheme + "://" + host
	}
	return scheme + "://" + host + "/" + path
}

// CheckFreshness reports whether the proof's timestamps are acceptable at the
// given server time.
func CheckFreshness(p *Proof, now time.Time) error {
	if !p.HasExp {
		return ErrStale
	}
	if now.Unix() >= p.EXP {
		return ErrStale
	}
	delta := now.Unix() - p.IAT
	if delta < -60 || delta > 60 {
		return ErrStale
	}
	return nil
}

// AccessTokenHash returns the base64url-encoded SHA-256 digest of the access
// token, the value carried in a proof's ath claim.
func AccessTokenHash(accessToken string) string {
	sum := sha256.Sum256([]byte(accessToken))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func checkSignature(alg string, pub crypto.PublicKey, signed, sig []byte) error {
	digest := sha256.Sum256(signed)
	switch k := pub.(type) {
	case *ecdsa.PublicKey:
		if alg != "ES256" {
			return ErrUnsupportedAlgorithm
		}
		if !ecdsa.VerifyASN1(k, digest[:], sig) {
			return ErrSignature
		}
		return nil
	case *rsa.PublicKey:
		if alg != "RS256" {
			return ErrUnsupportedAlgorithm
		}
		if err := rsa.VerifyPKCS1v15(k, crypto.SHA256, digest[:], sig); err != nil {
			return ErrSignature
		}
		return nil
	default:
		return ErrUnsupportedAlgorithm
	}
}

func decodeSegment(s string) ([]byte, error) {
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		data, err = base64.URLEncoding.DecodeString(s)
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}
