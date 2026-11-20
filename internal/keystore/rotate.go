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
	staged := make([]*Record, len(records))
	for i, rec := range records {
		if err := validateRecordShape(rec); err != nil {
			return Rotation{TargetVersion: target}, fmt.Errorf("%w: record %d: %w", ErrRotationFailed, i, err)
		}
		if rec.Alg != AlgAESGCM {
			return Rotation{TargetVersion: target}, fmt.Errorf("%w: record %d: %w", ErrRotationFailed, i, fmt.Errorf("%w: %q", ErrUnsupportedAlgorithm, rec.Alg))
		}
		if rec.KeyVersion == target {
			res.Skipped++
			continue
		}
		rotated, err := s.rewrapCopy(rec, target, targetAEAD)
		if err != nil {
			return Rotation{TargetVersion: target}, fmt.Errorf("%w: record %d: %w", ErrRotationFailed, i, err)
		}
		staged[i] = rotated
		res.Rotated++
	}

	// Nothing has been mutated yet. Commit the staged copies only after every
	// record in the batch has been successfully re-wrapped.
	for i, rotated := range staged {
		if rotated != nil {
			*records[i] = *rotated
		}
	}
	return res, nil
}

// rewrapCopy returns a copy of rec moved onto target. The input record is not
// modified. The DEK is unwrapped with the record's current KEK, the payload is
// re-sealed under the target header, and the DEK is wrapped with the target
// KEK.
func (s *Sealer) rewrapCopy(rec *Record, target int, targetAEAD cipher.AEAD) (*Record, error) {
	wrapVersion, wrapNonce, wrappedDEK, err := decodeWrappedDEK(rec.WrappedDEK, 12, 16)
	if err != nil {
		return nil, err
	}
	oldKEK, err := s.ring.KEK(wrapVersion)
	if err != nil {
		return nil, err
	}
	oldAEAD, err := newAEAD(oldKEK)
	if err != nil {
		return nil, err
	}
	dek, err := oldAEAD.Open(nil, wrapNonce, wrappedDEK, wrapAAD(rec.ID))
	if err != nil {
		return nil, fmt.Errorf("%w: unwrap dek", ErrAuthentication)
	}

	payloadAEAD, err := newAEAD(dek)
	if err != nil {
		return nil, err
	}
	plaintext, err := openPayload(payloadAEAD, rec, wrapVersion)
	if err != nil {
		return nil, err
	}

	out := *rec
	out.KeyVersion = target

	newNonce := make([]byte, payloadAEAD.NonceSize())
	if _, err := rand.Read(newNonce); err != nil {
		return nil, fmt.Errorf("kekseal: generate nonce: %w", err)
	}
	out.Nonce = newNonce
	out.Ciphertext = payloadAEAD.Seal(nil, newNonce, plaintext, BuildAAD(&out))

	newWrapNonce := make([]byte, targetAEAD.NonceSize())
	if _, err := rand.Read(newWrapNonce); err != nil {
		return nil, fmt.Errorf("kekseal: generate wrap nonce: %w", err)
	}
	out.WrappedDEK = encodeWrappedDEK(target, newWrapNonce,
		targetAEAD.Seal(nil, newWrapNonce, dek, wrapAAD(out.ID)))

	return &out, nil
}
