// Package jwk parses JSON Web Keys presented in DPoP proof headers and
// computes their RFC 7638 thumbprints.
package jwk

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
)

var (
	// ErrMalformed is returned when a JWK cannot be understood.
	ErrMalformed = errors.New("jwk: malformed key")
	// ErrUnsupportedKey is returned for key types or curves the server does not use.
	ErrUnsupportedKey = errors.New("jwk: unsupported key")
	// ErrPrivateKey is returned when a proof carries private key material.
	ErrPrivateKey = errors.New("jwk: private key material")
)

// Key is a parsed JWK. The members are kept as decoded JSON so that callers
// can inspect exactly what the client presented.
type Key struct {
	raw map[string]any
}

// Parse decodes a JWK document. Only public EC and RSA keys are accepted.
func Parse(data []byte) (*Key, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var members map[string]any
	if err := dec.Decode(&members); err != nil || members == nil {
		return nil, ErrMalformed
	}
	for _, name := range []string{"d", "p", "q", "dp", "dq", "qi", "k"} {
		if _, ok := members[name]; ok {
			return nil, ErrPrivateKey
		}
	}

	k := &Key{raw: members}
	switch k.Kty() {
	case "EC", "RSA":
		return k, nil
	default:
		return nil, ErrUnsupportedKey
	}
}

// Kty returns the key type member.
func (k *Key) Kty() string { return k.member("kty") }

// Alg returns the alg member, if the client included one.
func (k *Key) Alg() string { return k.member("alg") }

func (k *Key) member(name string) string {
	if k == nil || k.raw == nil {
		return ""
	}
	s, _ := k.raw[name].(string)
	return s
}

// PublicKey converts the JWK into a crypto.PublicKey suitable for signature
// verification.
func (k *Key) PublicKey() (crypto.PublicKey, error) {
	if k == nil || k.raw == nil {
		return nil, ErrMalformed
	}
	switch k.Kty() {
	case "EC":
		if k.member("crv") != "P-256" {
			return nil, ErrUnsupportedKey
		}
		x, err := decodeInt(k.member("x"))
		if err != nil {
			return nil, err
		}
		y, err := decodeInt(k.member("y"))
		if err != nil {
			return nil, err
		}
		curve := elliptic.P256()
		if x.Sign() <= 0 || y.Sign() <= 0 || !curve.IsOnCurve(x, y) {
			return nil, ErrMalformed
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
	case "RSA":
		n, err := decodeInt(k.member("n"))
		if err != nil {
			return nil, err
		}
		e, err := decodeInt(k.member("e"))
		if err != nil {
			return nil, err
		}
		if n.Sign() <= 0 || e.Sign() <= 0 || e.BitLen() > 31 {
			return nil, ErrMalformed
		}
		return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
	default:
		return nil, ErrUnsupportedKey
	}
}

func decodeInt(value string) (*big.Int, error) {
	if value == "" {
		return nil, ErrMalformed
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		data, err = base64.URLEncoding.DecodeString(value)
		if err != nil {
			return nil, ErrMalformed
		}
	}
	if len(data) == 0 {
		return nil, ErrMalformed
	}
	return new(big.Int).SetBytes(data), nil
}

// Thumbprint returns the base64url-encoded SHA-256 digest of the key's
// canonical JSON form, as specified by RFC 7638. The canonical form contains
// exactly the required members for the key type, in lexicographic order;
// members such as kid, alg, use and key_ops are omitted.
func Thumbprint(k *Key) (string, error) {
	if k == nil || k.raw == nil {
		return "", ErrMalformed
	}
	var canonical string
	switch k.Kty() {
	case "EC":
		crv, x, y := k.member("crv"), k.member("x"), k.member("y")
		if crv == "" || x == "" || y == "" {
			return "", ErrMalformed
		}
		canonical = `{"crv":` + strconv.Quote(crv) +
			`,"kty":"EC","x":` + strconv.Quote(x) +
			`,"y":` + strconv.Quote(y) + `}`
	case "RSA":
		e, n := k.member("e"), k.member("n")
		if e == "" || n == "" {
			return "", ErrMalformed
		}
		canonical = `{"e":` + strconv.Quote(e) +
			`,"kty":"RSA","n":` + strconv.Quote(n) + `}`
	default:
		return "", ErrUnsupportedKey
	}
	sum := sha256.Sum256([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// Matches reports whether two encoded thumbprints identify the same key.
// The comparison is exact: base64url is case-sensitive.
func Matches(a, b string) bool {
	return a == b
}
