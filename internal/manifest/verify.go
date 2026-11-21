package manifest

import (
	"crypto/ed25519"
	"errors"
	"fmt"

	"cborveil/internal/cbor"
)

// Errors reported by Verify and Select.
var (
	ErrSignatureInvalid = errors.New("manifest: signature verification failed")
	ErrNoMatchingTarget = errors.New("manifest: no target for this device model")
	ErrManifestInvalid  = errors.New("manifest: invalid")
)

// SigningBytes returns the byte string covered by the manifest signature. The
// fields describing the signature itself are not part of the covered content.
func SigningBytes(m *Manifest) ([]byte, error) {
	if m == nil || m.raw == nil {
		return nil, fmt.Errorf("%w: no decoded manifest", ErrManifestInvalid)
	}
	body := m.raw.Clone()
	body.Delete("sig")
	return cbor.Encode(body)
}

// Verify checks the manifest signature against pub.
func Verify(m *Manifest, pub ed25519.PublicKey) error {
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("%w: bad public key length %d", ErrManifestInvalid, len(pub))
	}
	if len(m.Sig) != ed25519.SignatureSize {
		return fmt.Errorf("%w: bad signature length %d", ErrManifestInvalid, len(m.Sig))
	}
	msg, err := SigningBytes(m)
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, msg, m.Sig) {
		return ErrSignatureInvalid
	}
	return nil
}
