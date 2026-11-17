// Package agent implements the edge side of seal relay: accept an envelope,
// verify its manifest, and materialize the secrets on disk.
package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StateFile is the name of the bookkeeping file kept in the agent root.
const StateFile = "state.json"

// State records what the agent last applied.
type State struct {
	Bundle    string   `json:"bundle"`
	Revision  uint64   `json:"revision"`
	Names     []string `json:"names"`
	AppliedAt int64    `json:"appliedAt"`
	ExpiresAt int64    `json:"expiresAt"`
}

// LoadState reads the bookkeeping file from root. A missing file yields an
// empty state.
func LoadState(root string) (*State, error) {
	data, err := os.ReadFile(filepath.Join(root, StateFile))
	if errors.Is(err, os.ErrNotExist) {
		return &State{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("agent: read state: %w", err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("agent: parse state: %w", err)
	}
	return &st, nil
}

// Save writes the bookkeeping file into root.
func (s *State) Save(root string) error {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return fmt.Errorf("agent: create root: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("agent: encode state: %w", err)
	}
	return writeFileAtomic(filepath.Join(root, StateFile), data)
}

func nowUnix(now func() time.Time) int64 {
	if now == nil {
		return time.Now().Unix()
	}
	return now().Unix()
}
