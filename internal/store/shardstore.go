// Package store persists encrypted shards on behalf of custody domains.
package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
)

// Errors reported by the shard store.
var (
	ErrShardNotFound     = errors.New("store: shard not found")
	ErrInvalidCustodyID  = errors.New("store: custody-id must not be empty")
	ErrInvalidShardID    = errors.New("store: shard-id must not be empty")
	ErrCiphertextCorrupt = errors.New("store: shard ciphertext failed integrity check")
	ErrCustodyMismatch   = errors.New("store: shard belongs to a different custody-id")
)

// Record is the metadata retained alongside an encrypted shard.
type Record struct {
	ShardID    string
	CustodyID  string
	SplitID    string
	Generation int
	Index      int
	Threshold  int
	Ciphertext []byte
}

// ShardStore keeps encrypted shards in memory. Ciphertexts are sealed with
// AES-GCM under a key derived from the store secret.
type ShardStore struct {
	mu      sync.RWMutex
	aead    cipher.AEAD
	records map[string]Record
}

// New builds a store protected by secret. The same secret is required to
// decrypt previously sealed shards.
func New(secret []byte) (*ShardStore, error) {
	if len(secret) == 0 {
		return nil, errors.New("store: store secret must not be empty")
	}
	key := sha256.Sum256(secret)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("store: create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("store: create aead: %w", err)
	}
	return &ShardStore{
		aead:    aead,
		records: make(map[string]Record),
	}, nil
}

// Put seals a shard payload under custodyID and stores it.
func (s *ShardStore) Put(custodyID, shardID, splitID string, generation, index, threshold int, plaintext []byte) error {
	if custodyID == "" {
		return ErrInvalidCustodyID
	}
	if shardID == "" {
		return ErrInvalidShardID
	}

	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("store: generate nonce: %w", err)
	}
	sealed := s.aead.Seal(nil, nonce, plaintext, []byte(custodyID))

	rec := Record{
		ShardID:    shardID,
		CustodyID:  custodyID,
		SplitID:    splitID,
		Generation: generation,
		Index:      index,
		Threshold:  threshold,
		Ciphertext: append(nonce, sealed...),
	}

	s.mu.Lock()
	s.records[shardID] = rec
	s.mu.Unlock()
	return nil
}

// Get returns the plaintext shard identified by shardID. The requesting
// custodyID must match the record's owning custody-id; otherwise an error is
// returned without decrypting or disclosing any shard data or metadata.
func (s *ShardStore) Get(custodyID, shardID string) ([]byte, Record, error) {
	if custodyID == "" {
		return nil, Record{}, ErrInvalidCustodyID
	}
	if shardID == "" {
		return nil, Record{}, ErrInvalidShardID
	}

	s.mu.RLock()
	rec, ok := s.records[shardID]
	s.mu.RUnlock()
	if !ok {
		return nil, Record{}, ErrShardNotFound
	}
	if rec.CustodyID != custodyID {
		return nil, Record{}, ErrCustodyMismatch
	}

	nonceSize := s.aead.NonceSize()
	if len(rec.Ciphertext) < nonceSize {
		return nil, Record{}, ErrCiphertextCorrupt
	}
	nonce := rec.Ciphertext[:nonceSize]
	body := rec.Ciphertext[nonceSize:]

	plaintext, err := s.aead.Open(nil, nonce, body, []byte(rec.CustodyID))
	if err != nil {
		return nil, Record{}, ErrCiphertextCorrupt
	}
	return plaintext, rec, nil
}

// List returns the records owned by custodyID.
func (s *ShardStore) List(custodyID string) []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Record, 0, len(s.records))
	for _, rec := range s.records {
		if rec.CustodyID == custodyID {
			out = append(out, rec)
		}
	}
	return out
}

// Len reports the number of shards held by the store.
func (s *ShardStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}
