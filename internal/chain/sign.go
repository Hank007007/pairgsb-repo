package chain

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const checkpointTag = "witnesschain/v1/checkpoint"

// Checkpoint is the signed statement about one sealed segment.
//
// Serialization order follows docs/chain-format.md §7.1.
type Checkpoint struct {
	Version        int    `json:"version"`
	Segment        uint64 `json:"segment"`
	Count          uint64 `json:"count"`
	FirstSeq       uint64 `json:"first_seq"`
	LastSeq        uint64 `json:"last_seq"`
	Root           string `json:"root"`
	PrevCheckpoint string `json:"prev_checkpoint"`
	KeyID          string `json:"key_id"`
	Signature      string `json:"signature"`
}

// SignedMessage returns the exact bytes covered by the signature.
//
// Defined in docs/chain-format.md §7.4.
func (c *Checkpoint) SignedMessage() []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", checkpointTag)
	fmt.Fprintf(&b, "segment=%d\n", c.Segment)
	fmt.Fprintf(&b, "count=%d\n", c.Count)
	fmt.Fprintf(&b, "first_seq=%d\n", c.FirstSeq)
	fmt.Fprintf(&b, "last_seq=%d\n", c.LastSeq)
	fmt.Fprintf(&b, "root=%s\n", c.Root)
	fmt.Fprintf(&b, "prev_checkpoint=%s\n", c.PrevCheckpoint)
	fmt.Fprintf(&b, "key_id=%s\n", c.KeyID)
	return []byte(b.String())
}

// Hash returns the checkpoint hash defined in §7.5.
func (c *Checkpoint) Hash() string {
	sum := sha256.Sum256(c.SignedMessage())
	return hex.EncodeToString(sum[:])
}

// Sign attaches an Ed25519 signature over SignedMessage.
func (c *Checkpoint) Sign(priv ed25519.PrivateKey, keyID string) {
	c.KeyID = keyID
	c.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, c.SignedMessage()))
}

// Verify checks the signature against pub.
func (c *Checkpoint) Verify(pub ed25519.PublicKey) bool {
	if c.Signature == "" || len(pub) != ed25519.PublicKeySize {
		return false
	}
	sig, err := base64.StdEncoding.DecodeString(c.Signature)
	if err != nil {
		return false
	}
	return ed25519.Verify(pub, c.SignedMessage(), sig)
}

// KeyPair is a loaded signing key.
type KeyPair struct {
	ID  string
	Pub ed25519.PublicKey
	Prv ed25519.PrivateKey
}

// GenerateKeyPair creates a fresh Ed25519 key with the given identifier.
func GenerateKeyPair(id string) (*KeyPair, error) {
	pub, prv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &KeyPair{ID: id, Pub: pub, Prv: prv}, nil
}

// SaveKeys writes <keyID>.key and <keyID>.pub under dir (which is created).
func SaveKeys(dir string, kp *KeyPair) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	prvDER, err := x509.MarshalPKCS8PrivateKey(kp.Prv)
	if err != nil {
		return err
	}
	prvPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: prvDER})
	if err := os.WriteFile(filepath.Join(dir, kp.ID+".key"), prvPEM, 0o600); err != nil {
		return err
	}
	pubDER, err := x509.MarshalPKIXPublicKey(kp.Pub)
	if err != nil {
		return err
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	return os.WriteFile(filepath.Join(dir, kp.ID+".pub"), pubPEM, 0o644)
}

// LoadPrivateKey reads keys/<id>.key.
func LoadPrivateKey(dir, id string) (ed25519.PrivateKey, error) {
	b, err := os.ReadFile(filepath.Join(dir, id+".key"))
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("key %q: no PEM block", id)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	prv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key %q: not an Ed25519 private key", id)
	}
	return prv, nil
}

// LoadPublicKey reads keys/<id>.pub.
func LoadPublicKey(dir, id string) (ed25519.PublicKey, error) {
	b, err := os.ReadFile(filepath.Join(dir, id+".pub"))
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("key %q: no PEM block", id)
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	pub, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key %q: not an Ed25519 public key", id)
	}
	return pub, nil
}

// KeySet is a lazy cache of public keys in a key directory.
type KeySet struct {
	Dir   string
	cache map[string]ed25519.PublicKey
}

// NewKeySet returns a key set rooted at dir.
func NewKeySet(dir string) *KeySet {
	return &KeySet{Dir: dir, cache: map[string]ed25519.PublicKey{}}
}

// Public resolves a key identifier to a public key.
func (ks *KeySet) Public(id string) (ed25519.PublicKey, error) {
	if pub, ok := ks.cache[id]; ok {
		return pub, nil
	}
	pub, err := LoadPublicKey(ks.Dir, id)
	if err != nil {
		return nil, err
	}
	ks.cache[id] = pub
	return pub, nil
}

// IDs lists key identifiers that have a public key file, sorted.
func (ks *KeySet) IDs() ([]string, error) {
	entries, err := os.ReadDir(ks.Dir)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".pub") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(e.Name(), ".pub"))
	}
	sort.Strings(ids)
	return ids, nil
}

// NewestKey returns the alphabetically last key identifier, which is the
// rotation convention used by witnessctl: identifiers are date-stamped.
func (ks *KeySet) NewestKey() (string, error) {
	ids, err := ks.IDs()
	if err != nil {
		return "", err
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("no keys in %s", ks.Dir)
	}
	return ids[len(ids)-1], nil
}
