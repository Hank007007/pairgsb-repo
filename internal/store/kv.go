// Package store holds the in-memory secret table loaded at start-up.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
)

// Secret is one entry of the secret table.
type Secret struct {
	Path    string `json:"path"`
	Value   string `json:"value"`
	Backend string `json:"backend"`
}

// fileShape mirrors the on-disk JSON document.
type fileShape struct {
	Secrets []Secret `json:"secrets"`
}

// KV is an immutable-after-load view of the secret table. Reads are
// concurrency-safe.
type KV struct {
	mu      sync.RWMutex
	secrets map[string]Secret
}

// New returns an empty table.
func New() *KV {
	return &KV{secrets: make(map[string]Secret)}
}

// LoadFile reads a JSON secret file and replaces the table contents with it.
func (kv *KV) LoadFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read secrets file: %w", err)
	}
	var doc fileShape
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse secrets file: %w", err)
	}
	next := make(map[string]Secret, len(doc.Secrets))
	for i, s := range doc.Secrets {
		if s.Path == "" {
			return fmt.Errorf("secrets[%d]: path is required", i)
		}
		if s.Value == "" {
			return fmt.Errorf("secrets[%d] (%s): value is required", i, s.Path)
		}
		if s.Backend == "" {
			return fmt.Errorf("secrets[%d] (%s): backend is required", i, s.Path)
		}
		if _, dup := next[s.Path]; dup {
			return fmt.Errorf("secrets[%d]: duplicate path %q", i, s.Path)
		}
		next[s.Path] = s
	}
	kv.mu.Lock()
	kv.secrets = next
	kv.mu.Unlock()
	return nil
}

// Get looks up a secret by exact path.
func (kv *KV) Get(path string) (Secret, bool) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	s, ok := kv.secrets[path]
	return s, ok
}

// Len reports the number of loaded secrets.
func (kv *KV) Len() int {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	return len(kv.secrets)
}

// Paths lists the loaded paths in lexical order.
func (kv *KV) Paths() []string {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	out := make([]string, 0, len(kv.secrets))
	for p := range kv.secrets {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
