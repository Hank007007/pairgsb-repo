package resolve

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// KVProvider serves secrets from a JSON firmware file shaped as
//
//	{"default": {"billing/api_key": "..."}, "staging": {...}}
//
// Keys are looked up in the requested scope first and then in "default".
type KVProvider struct {
	scopes map[string]map[string]string
}

// NewKVProvider loads a firmware file.
func NewKVProvider(path string) (*KVProvider, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("kv: read firmware: %w", err)
	}
	var scopes map[string]map[string]string
	if err := json.Unmarshal(data, &scopes); err != nil {
		return nil, fmt.Errorf("kv: parse firmware: %w", err)
	}
	return NewKVProviderFromMap(scopes), nil
}

// NewKVProviderFromMap builds a provider from an in-memory firmware map.
func NewKVProviderFromMap(scopes map[string]map[string]string) *KVProvider {
	copied := make(map[string]map[string]string, len(scopes))
	for scope, entries := range scopes {
		m := make(map[string]string, len(entries))
		for k, v := range entries {
			m[k] = v
		}
		copied[scope] = m
	}
	return &KVProvider{scopes: copied}
}

// Scheme implements Provider.
func (k *KVProvider) Scheme() string { return SchemeKV }

// Scopes lists the scopes present in the firmware.
func (k *KVProvider) Scopes() []string {
	out := make([]string, 0, len(k.scopes))
	for name := range k.scopes {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Resolve looks the key up in scope, falling back to the default scope.
func (k *KVProvider) Resolve(ctx context.Context, scope string, ref Ref) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	for _, name := range lookupScopes(scope) {
		if values, ok := k.scopes[name]; ok {
			if value, ok := values[ref.Path]; ok {
				return value, nil
			}
		}
	}
	return "", fmt.Errorf("kv: key %q not found in scope %q", ref.Path, scope)
}

func lookupScopes(scope string) []string {
	if scope == "" || scope == "default" {
		return []string{"default"}
	}
	return []string{scope, "default"}
}
