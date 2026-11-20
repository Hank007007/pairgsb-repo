package keystore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
)

// AlgAESGCM is the record algorithm defined by the envelope format.
const AlgAESGCM = "aes-gcm"

const dekSize = 32

// wrapVersionSize is the number of leading bytes of WrappedDEK that hold the
// KEK version the DEK was wrapped under, big-endian. A reader selects the KEK
// from this prefix.
const wrapVersionSize = 4

// Sealer produces and consumes sealed records with a fixed key ring.
type Sealer struct {
	ring *Ring
}

// NewSealer returns a Sealer backed by ring.
func NewSealer(ring *Ring) *Sealer {
	return &Sealer{ring: ring}
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// encodeWrappedDEK packs the KEK version, the wrap nonce, and the sealed DEK.
func encodeWrappedDEK(version int, nonce, sealed []byte) []byte {
	out := make([]byte, wrapVersionSize+len(nonce)+len(sealed))
	binary.BigEndian.PutUint32(out[:wrapVersionSize], uint32(version))
	copy(out[wrapVersionSize:], nonce)
	copy(out[wrapVersionSize+len(nonce):], sealed)
	return out
}

func decodeWrappedDEK(b []byte, nonceSize, overhead int) (version int, nonce, sealed []byte, err error) {
	if len(b) < wrapVersionSize+nonceSize+overhead {
		return 0, nil, nil, ErrInvalidRecord
	}
	version = int(binary.BigEndian.Uint32(b[:wrapVersionSize]))
	nonce = b[wrapVersionSize : wrapVersionSize+nonceSize]
	sealed = b[wrapVersionSize+nonceSize:]
	return version, nonce, sealed, nil
}

// Seal encrypts plaintext for tenant under a fresh DEK, wraps that DEK with
// the ring's active KEK version, and returns the resulting record.
func (s *Sealer) Seal(tenant, id string, plaintext []byte) (*Record, error) {
	if tenant == "" {
		return nil, errors.Join(ErrInvalidRecord, errors.New("missing tenant"))
	}
	if id == "" {
		return nil, errors.Join(ErrInvalidRecord, errors.New("missing id"))
	}
	if s.ring == nil {
		return nil, ErrEmptyRing
	}
	version, kek, err := s.ring.ActiveKEK()
	if err != nil {
		return nil, err
	}

	dek := make([]byte, dekSize)
	if _, err := rand.Read(dek); err != nil {
		return nil, fmt.Errorf("kekseal: generate dek: %w", err)
	}

	rec := &Record{
		ID:         id,
		Tenant:     tenant,
		KeyVersion: version,
		Alg:        AlgAESGCM,
	}

	payloadAEAD, err := newAEAD(dek)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, payloadAEAD.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("kekseal: generate nonce: %w", err)
	}
	rec.Nonce = nonce
	rec.Ciphertext = payloadAEAD.Seal(nil, nonce, plaintext, BuildAAD(rec))

	kekAEAD, err := newAEAD(kek)
	if err != nil {
		return nil, err
	}
	wrapNonce := make([]byte, kekAEAD.NonceSize())
	if _, err := rand.Read(wrapNonce); err != nil {
		return nil, fmt.Errorf("kekseal: generate wrap nonce: %w", err)
	}
	wrapped := kekAEAD.Seal(nil, wrapNonce, dek, wrapAAD(rec.ID))
	rec.WrappedDEK = encodeWrappedDEK(version, wrapNonce, wrapped)

	return rec, nil
}

// Open decrypts rec, returning the plaintext.
func (s *Sealer) Open(rec *Record) ([]byte, error) {
	if err := validateRecordShape(rec); err != nil {
		return nil, err
	}
	if rec.Alg != AlgAESGCM {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedAlgorithm, rec.Alg)
	}
	return s.openSealed(rec)
}

func (s *Sealer) openSealed(rec *Record) ([]byte, error) {
	if s.ring == nil {
		return nil, ErrEmptyRing
	}
	wrapVersion, wrapNonce, wrappedDEK, err := decodeWrappedDEK(rec.WrappedDEK, 12, 16)
	if err != nil {
		return nil, err
	}
	kek, err := s.ring.KEK(wrapVersion)
	if err != nil {
		return nil, err
	}
	kekAEAD, err := newAEAD(kek)
	if err != nil {
		return nil, err
	}
	dek, err := kekAEAD.Open(nil, wrapNonce, wrappedDEK, wrapAAD(rec.ID))
	if err != nil {
		return nil, fmt.Errorf("%w: unwrap dek", ErrAuthentication)
	}

	payloadAEAD, err := newAEAD(dek)
	if err != nil {
		return nil, err
	}
	return openPayload(payloadAEAD, rec, wrapVersion)
}

// openPayload decrypts rec.Ciphertext. Records sealed before the header was
// bound into the AAD are accepted only when their header still matches the
// wrap version that unwrapped their DEK.
func openPayload(payloadAEAD cipher.AEAD, rec *Record, wrapVersion int) ([]byte, error) {
	plaintext, err := payloadAEAD.Open(nil, rec.Nonce, rec.Ciphertext, BuildAAD(rec))
	if err == nil {
		return plaintext, nil
	}
	if rec.KeyVersion == wrapVersion {
		plaintext, legacyErr := payloadAEAD.Open(nil, rec.Nonce, rec.Ciphertext, legacyAAD(rec))
		if legacyErr == nil {
			return plaintext, nil
		}
	}
	return nil, fmt.Errorf("%w: decrypt payload", ErrAuthentication)
}

// wrapAAD returns the additional data used when sealing a DEK to its KEK.
func wrapAAD(id string) []byte {
	return []byte("kekseal-dek|" + id)
}
