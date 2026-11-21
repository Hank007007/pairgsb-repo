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
	IAT int64  `json:"iat"`
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

	out := &Proof{
		JTI:     p.JTI,
		HTM:     p.HTM,
		HTU:     p.HTU,
		ATH:     p.ATH,
		IAT:     p.IAT,
		Key:     key,
		Alg:     h.Alg,
		Request: req,
	}
	if p.EXP != nil {
		out.EXP = *p.EXP
		out.HasExp = true
	}
	return out, nil
}

// CheckFreshness reports whether the proof's timestamps are acceptable at the
// given server time.
func CheckFreshness(p *Proof, now time.Time) error {
	if !p.HasExp {
		return nil
	}
	reference := now
	if p.IAT != 0 {
		reference = time.Unix(p.IAT, 0)
	}
	if reference.Unix() >= p.EXP {
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

func htuMatches(expected, actual string) bool {
	return strings.HasPrefix(actual, expected)
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
