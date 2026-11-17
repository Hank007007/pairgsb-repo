package agent

import (
	"bytes"
	"crypto/ecdh"
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	"sealrelay/internal/bundle"
	"sealrelay/internal/derive"
	"sealrelay/internal/envelope"
	"sealrelay/internal/manifest"
	"sealrelay/internal/nodekeys"
	"sealrelay/internal/seal"
)

// Receiver applies control-plane envelopes on one edge node.
type Receiver struct {
	Root    string
	NodeID  string
	PrivKey *ecdh.PrivateKey
	SignPub ed25519.PublicKey
	Now     func() time.Time
}

// NewReceiver validates the local identity and signing key.
func NewReceiver(root, nodeID string, privRaw []byte, signPub ed25519.PublicKey) (*Receiver, error) {
	priv, err := nodekeys.PrivateKey(privRaw)
	if err != nil {
		return nil, err
	}
	if len(signPub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("agent: signing key must be %d bytes", ed25519.PublicKeySize)
	}
	if nodeID != nodekeys.ID(priv.PublicKey().Bytes()) {
		return nil, errors.New("agent: node id does not match private key")
	}
	return &Receiver{
		Root:    root,
		NodeID:  nodeID,
		PrivKey: priv,
		SignPub: signPub,
		Now:     time.Now,
	}, nil
}

// Accept verifies an envelope and, if it is admissible, materializes it.
func (r *Receiver) Accept(env *envelope.Envelope) (*State, error) {
	if env == nil {
		return nil, errors.New("agent: nil envelope")
	}
	if err := manifest.VerifyManifest(&env.Manifest, r.SignPub); err != nil {
		return nil, err
	}
	if env.Manifest.Recipient != r.NodeID {
		return nil, fmt.Errorf("agent: envelope addressed to %q, not %q", env.Manifest.Recipient, r.NodeID)
	}
	if nowUnix(r.Now) >= env.Manifest.ExpiresAt {
		return nil, fmt.Errorf("agent: envelope expired at %d", env.Manifest.ExpiresAt)
	}

	state, err := LoadState(r.Root)
	if err != nil {
		return nil, err
	}
	if err := manifest.AcceptRevision(state.Revision, env.Manifest.Payload.Revision); err != nil {
		return nil, err
	}

	secrets, err := r.open(env)
	if err != nil {
		return nil, err
	}
	if err := checkNames(env.Manifest.Payload, secrets); err != nil {
		return nil, err
	}

	if err := ApplyBundle(r.Root, secrets); err != nil {
		return nil, err
	}

	next := &State{
		Bundle:    env.Manifest.Payload.Bundle,
		Revision:  env.Manifest.Payload.Revision,
		Names:     namesOf(secrets),
		AppliedAt: nowUnix(r.Now),
		ExpiresAt: env.Manifest.ExpiresAt,
	}
	if err := next.Save(r.Root); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Receiver) open(env *envelope.Envelope) ([]bundle.Secret, error) {
	eph, err := nodekeys.PublicKey(env.EphemeralPublic)
	if err != nil {
		return nil, err
	}
	shared, err := r.PrivKey.ECDH(eph)
	if err != nil {
		return nil, fmt.Errorf("agent: ecdh: %w", err)
	}
	key, err := derive.DeriveKey(shared, env.Manifest.Payload.Bundle, r.PrivKey.PublicKey().Bytes())
	if err != nil {
		return nil, err
	}
	plaintext, err := seal.Decrypt(key, env.Nonce, env.Ciphertext)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(digestOf(plaintext), env.Manifest.Payload.Digest) {
		return nil, errors.New("agent: bundle digest mismatch")
	}
	return bundle.Decode(plaintext)
}

func checkNames(p manifest.Payload, secrets []bundle.Secret) error {
	have := namesOf(secrets)
	if len(have) != len(p.Names) {
		return errors.New("agent: manifest name list does not match bundle contents")
	}
	for i := range have {
		if have[i] != p.Names[i] {
			return errors.New("agent: manifest name list does not match bundle contents")
		}
	}
	return nil
}

func namesOf(secrets []bundle.Secret) []string {
	names := make([]string, 0, len(secrets))
	for _, sec := range secrets {
		names = append(names, sec.Name)
	}
	return names
}
