// Package envelope defines the wire representation of a sealed bundle.
package envelope

import (
	"encoding/json"
	"fmt"

	"sealrelay/internal/manifest"
)

// Version is the current envelope format version.
const Version = 1

// Envelope is what the control plane hands to an edge node.
type Envelope struct {
	Version         int               `json:"version"`
	EphemeralPublic []byte            `json:"ephemeralPublic"`
	Nonce           []byte            `json:"nonce"`
	Ciphertext      []byte            `json:"ciphertext"`
	Manifest        manifest.Manifest `json:"manifest"`
}

// Marshal encodes the envelope for transport.
func (e *Envelope) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// Parse decodes an envelope received from the control plane.
func Parse(data []byte) (*Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("envelope: decode: %w", err)
	}
	if e.Version != Version {
		return nil, fmt.Errorf("envelope: unsupported version %d", e.Version)
	}
	return &e, nil
}
