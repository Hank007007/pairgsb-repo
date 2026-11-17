package keyring

import (
	"encoding/json"
	"fmt"
	"os"
)

type diskKey struct {
	Version uint32 `json:"version"`
	Status  Status `json:"status"`
	KEK     []byte `json:"kek"`
}

type diskKeyring struct {
	Format int       `json:"format"`
	Keys   []diskKey `json:"keys"`
}

const documentFormat = 1

// Load reads a keyring document from path.
func Load(path string) (*Keyring, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	kr, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return kr, nil
}

// Parse decodes a keyring document.
func Parse(data []byte) (*Keyring, error) {
	var doc diskKeyring
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("decode document: %w", err)
	}
	if len(doc.Keys) == 0 {
		return nil, ErrEmpty
	}

	seen := make(map[uint32]struct{}, len(doc.Keys))
	keys := make([]Key, 0, len(doc.Keys))
	for _, dk := range doc.Keys {
		if dk.Version == 0 {
			return nil, ErrInvalidVersion
		}
		if _, ok := seen[dk.Version]; ok {
			return nil, fmt.Errorf("%w: %d", ErrDuplicateVersion, dk.Version)
		}
		seen[dk.Version] = struct{}{}

		switch dk.Status {
		case StatusActive, StatusRetired:
		default:
			return nil, fmt.Errorf("%w: %q", ErrInvalidStatus, dk.Status)
		}
		if len(dk.KEK) != KeySize {
			return nil, fmt.Errorf("%w: version %d has %d bytes", ErrInvalidKeySize, dk.Version, len(dk.KEK))
		}

		keys = append(keys, Key{
			Version:  dk.Version,
			Status:   dk.Status,
			Material: dk.KEK,
		})
	}

	return New(keys), nil
}

// Marshal encodes a keyring document.
func Marshal(r *Keyring) ([]byte, error) {
	keys := r.Keys()
	doc := diskKeyring{Format: documentFormat, Keys: make([]diskKey, 0, len(keys))}
	for _, k := range keys {
		doc.Keys = append(doc.Keys, diskKey{
			Version: k.Version,
			Status:  k.Status,
			KEK:     k.Material,
		})
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode document: %w", err)
	}
	return append(data, '\n'), nil
}

// Save writes a keyring document to path.
func Save(path string, r *Keyring) error {
	data, err := Marshal(r)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
