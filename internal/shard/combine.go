package shard

import "errors"

// Errors reported when shards do not form a single split batch.
var (
	ErrEmptyShardSplitID  = errors.New("shard: shard split-id must not be empty")
	ErrNegativeGeneration = errors.New("shard: shard generation must not be negative")
	ErrMismatchedSplitID  = errors.New("shard: shards belong to different split-ids")
	ErrMismatchedGen      = errors.New("shard: shards belong to different generations")
)

// Combine reconstructs the master key from a set of shards.
//
// Every input shard must come from the same split batch: identical split-id
// and generation. The batch identity check runs before any reconstruction
// math, so a mixed batch is rejected without producing candidate key bytes.
func Combine(shards []Shard) ([]byte, error) {
	if len(shards) == 0 {
		return nil, ErrNoShards
	}

	splitID := shards[0].SplitID
	generation := shards[0].Generation
	for _, s := range shards {
		if s.SplitID == "" {
			return nil, ErrEmptyShardSplitID
		}
		if s.Generation < 0 {
			return nil, ErrNegativeGeneration
		}
		if s.SplitID != splitID {
			return nil, ErrMismatchedSplitID
		}
		if s.Generation != generation {
			return nil, ErrMismatchedGen
		}
	}

	threshold := shards[0].Threshold
	if threshold <= 0 {
		return nil, ErrInvalidThreshold
	}
	if len(shards) < threshold {
		return nil, ErrInsufficientShards
	}

	length := len(shards[0].Data)
	seen := make(map[int]struct{}, len(shards))
	for _, s := range shards {
		if s.Index < 1 || s.Index > 255 {
			return nil, ErrIndexOutOfRange
		}
		if len(s.Data) != length {
			return nil, ErrLengthMismatch
		}
		if _, ok := seen[s.Index]; ok {
			return nil, ErrDuplicateIndex
		}
		seen[s.Index] = struct{}{}
	}

	out := make([]byte, length)
	for pos := 0; pos < length; pos++ {
		var acc byte
		for i := range shards {
			weight := byte(1)
			for j := range shards {
				if i == j {
					continue
				}
				denominator := byte(shards[i].Index ^ shards[j].Index)
				if denominator == 0 {
					return nil, ErrDuplicateIndex
				}
				weight = mul(weight, div(byte(shards[j].Index), denominator))
			}
			acc ^= mul(shards[i].Data[pos], weight)
		}
		out[pos] = acc
	}
	return out, nil
}
