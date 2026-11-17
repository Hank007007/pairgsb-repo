package keyring

import (
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

// MinMasterSecretSize is the smallest master secret accepted by DeriveKEK.
const MinMasterSecretSize = 32

// DeriveKEK expands master into the KEK for the given key version.
//
// The returned key is suitable for AES-256-GCM wrapping.
func DeriveKEK(master []byte, version uint32) ([]byte, error) {
	if len(master) < MinMasterSecretSize {
		return nil, fmt.Errorf("master secret must be at least %d bytes", MinMasterSecretSize)
	}
	if version == 0 {
		return nil, ErrInvalidVersion
	}

	var versionBytes [4]byte
	binary.BigEndian.PutUint32(versionBytes[:], version)

	saltInput := append([]byte("kekring/kek/salt/v1/"), versionBytes[:]...)
	salt := sha256.Sum256(saltInput)
	info := string(append([]byte("kekring/kek/info/v1/"), versionBytes[:]...))

	kek, err := hkdf.Key(sha256.New, master, salt[:], info, KeySize)
	if err != nil {
		return nil, fmt.Errorf("derive key version: %w", err)
	}
	if len(kek) != KeySize {
		return nil, errors.New("derived key has an unexpected size")
	}
	return kek, nil
}
