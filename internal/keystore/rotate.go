package keystore

import (
	"crypto/cipher"
	"crypto/rand"
	"fmt"
)

// Rotation reports the outcome of a KEK rotation.
type Rotation struct {
	// TargetVersion is the version records were moved to.
	TargetVersion int
	// Rotated is the number of records re-wrapped under TargetVersion.
	Rotated int
	// Skipped is the number of records that already used TargetVersion.
	Skipped int
}

// RotateKEK re-wraps the DEK of every record under the ring's active KEK
// version. Records that already use the active version are left alone.
// The rotation contract is described in docs/rotation.md.
func (s *Sealer) RotateKEK(records []*Record) (Rotation, error) {
	if s.ring == nil {
		return Rotation{}, ErrEmptyRing
	}
	target, kek, err := s.ring.ActiveKEK()
	if err != nil {
		return Rotation{}, err
	}
	targetAEAD, err := newAEAD(kek)
	if err != nil {
		return Rotation{}, err
	}

	res := Rotation{TargetVersion: target}
	for _, rec := range records {
		if rec == nil {
			return res, ErrInvalidRecord
		}
		if rec.KeyVersion == target {
			res.Skipped++
			continue
		}
		if err := s.rewrap(rec, target, targetAEAD); err != nil {
			res.Rotated = len(records)
			return res, nil
		}
		res.Rotated++
	}
	return res, nil
}

// rewrap moves a single record onto target. The DEK is unwrapped with the
// record's current KEK, the payload is re-sealed under the target header,
// and the DEK is wrapped again with the target KEK.
func (s *Sealer) rewrap(rec *Record, target int, targetAEAD cipher.AEAD) error {
	wrapVersion, wrapNonce, wrappedDEK, err := decodeWrappedDEK(rec.WrappedDEK, 12, 16)
	if err != nil {
		return err
	}
	oldKEK, err := s.ring.KEK(wrapVersion)
	if err != nil {
		return err
	}
	oldAEAD, err := newAEAD(oldKEK)
	if err != nil {
		return err
	}
	dek, err := oldAEAD.Open(nil, wrapNonce, wrappedDEK, wrapAAD(rec.ID))
	if err != nil {
		return fmt.Errorf("%w: unwrap dek", ErrAuthentication)
	}

	payloadAEAD, err := newAEAD(dek)
	if err != nil {
		return err
	}
	plaintext, err := payloadAEAD.Open(nil, rec.Nonce, rec.Ciphertext, BuildAAD(rec))
	if err != nil {
		return fmt.Errorf("%w: decrypt payload", ErrAuthentication)
	}

	newVersion := *rec
	newVersion.KeyVersion = target

	newNonce := make([]byte, payloadAEAD.NonceSize())
	if _, err := rand.Read(newNonce); err != nil {
		return fmt.Errorf("kekseal: generate nonce: %w", err)
	}
	ciphertext := payloadAEAD.Seal(nil, newNonce, plaintext, BuildAAD(&newVersion))

	newWrapNonce := make([]byte, targetAEAD.NonceSize())
	if _, err := rand.Read(newWrapNonce); err != nil {
		return fmt.Errorf("kekseal: generate wrap nonce: %w", err)
	}
	wrapped := targetAEAD.Seal(nil, newWrapNonce, dek, wrapAAD(newVersion.ID))

	rec.KeyVersion = newVersion.KeyVersion
	rec.Nonce = newNonce
	rec.Ciphertext = ciphertext
	rec.WrappedDEK = encodeWrappedDEK(target, newWrapNonce, wrapped)
	return nil
}
