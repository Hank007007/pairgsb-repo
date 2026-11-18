// Package shard implements threshold splitting and reconstruction of a
// master key into independently held shards.
package shard

import (
	"crypto/rand"
	"errors"
	"fmt"
)

// Shard is a single share of a master key produced by Split.
//
// All shards belonging to the same splitting operation share SplitID and
// Generation. Index identifies the evaluation point of the share and is
// unique within a split.
type Shard struct {
	ShardID    string
	SplitID    string
	Generation int
	Index      int
	Threshold  int
	Data       []byte
}

// Errors reported by Split and Combine.
var (
	ErrEmptySecret        = errors.New("shard: master secret must not be empty")
	ErrInvalidCounts      = errors.New("shard: threshold and total must satisfy 1 <= threshold <= total <= 255")
	ErrEmptySplitID       = errors.New("shard: split-id must not be empty")
	ErrInvalidGeneration  = errors.New("shard: generation must be non-negative")
	ErrInsufficientShards = errors.New("shard: not enough shards to reconstruct")
	ErrInvalidThreshold   = errors.New("shard: shard threshold must be positive")
	ErrDuplicateIndex     = errors.New("shard: duplicate shard index")
	ErrIndexOutOfRange    = errors.New("shard: shard index out of range")
	ErrLengthMismatch     = errors.New("shard: shard payload lengths differ")
	ErrNoShards           = errors.New("shard: no shards supplied")
)

// Split divides secret into total shards requiring threshold shares for
// reconstruction. The returned shards are labelled with splitID and
// generation and carry unique ids derived from splitID and their index.
func Split(secret []byte, total, threshold int, splitID string, generation int) ([]Shard, error) {
	if len(secret) == 0 {
		return nil, ErrEmptySecret
	}
	if threshold < 1 || total < threshold || total > 255 {
		return nil, ErrInvalidCounts
	}
	if splitID == "" {
		return nil, ErrEmptySplitID
	}
	if generation < 0 {
		return nil, ErrInvalidGeneration
	}

	shards := make([]Shard, total)
	coeffs := make([]byte, threshold)
	for i := range shards {
		shards[i] = Shard{
			ShardID:    fmt.Sprintf("%s-%d", splitID, i+1),
			SplitID:    splitID,
			Generation: generation,
			Index:      i + 1,
			Threshold:  threshold,
			Data:       make([]byte, len(secret)),
		}
	}

	for pos := range secret {
		coeffs[0] = secret[pos]
		if threshold > 1 {
			if _, err := rand.Read(coeffs[1:]); err != nil {
				return nil, fmt.Errorf("shard: generate coefficients: %w", err)
			}
		}
		for i := range shards {
			shards[i].Data[pos] = evalPolynomial(coeffs, byte(shards[i].Index))
		}
	}
	return shards, nil
}

func evalPolynomial(coeffs []byte, x byte) byte {
	var acc byte
	for i := len(coeffs) - 1; i >= 0; i-- {
		acc = mul(acc, x) ^ coeffs[i]
	}
	return acc
}
