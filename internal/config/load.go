package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"refgate/internal/resolve"
)

// DefaultScope is used when Options.Scope is empty.
const DefaultScope = "default"

// Options describes a load.
type Options struct {
	// BasePath is the base configuration file. Required.
	BasePath string
	// OverlayPath is the environment overlay. Optional.
	OverlayPath string
	// Scope names the environment, e.g. "staging". Defaults to DefaultScope.
	Scope string
	// Vars supplies the values substituted for ${NAME} placeholders.
	Vars map[string]string
	// Resolver resolves secret references. Required.
	Resolver *resolve.Resolver
	// Context is used for provider calls. Defaults to context.Background.
	Context context.Context
}

// Load reads the base document, merges the optional overlay, expands ${...}
// placeholders, resolves references and returns the effective Config.
func Load(opts Options) (*Config, error) {
	if strings.TrimSpace(opts.BasePath) == "" {
		return nil, errors.New("config: base path is required")
	}
	if opts.Resolver == nil {
		return nil, errors.New("config: resolver is required")
	}
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}

	base, err := readDocument(opts.BasePath)
	if err != nil {
		return nil, err
	}
	doc := base
	if opts.OverlayPath != "" {
		overlay, err := readDocument(opts.OverlayPath)
		if err != nil {
			return nil, err
		}
		doc = Merge(base, overlay)
	}

	scope := opts.Scope
	if scope == "" {
		scope = DefaultScope
	}
	cfg := newConfig(scope)
	keys := make([]string, 0, len(doc))
	for key := range doc {
		keys = append(keys, key)
	}
	sortStrings(keys)

	for _, key := range keys {
		switch value := doc[key].(type) {
		case string:
			resolved, isSecret, err := resolveValue(ctx, opts.Resolver, scope, value)
			if err != nil {
				return nil, fmt.Errorf("config: key %q: %w", key, err)
			}
			expanded, err := expandVars(resolved, opts.Vars)
			if err != nil {
				return nil, fmt.Errorf("config: key %q: %w", key, err)
			}
			m := meta{}
			if isSecret {
				m.secret = true
				m.ref = value
			}
			cfg.put(key, expanded, m)
		case []string:
			items := make([]string, len(value))
			m := meta{}
			for i, item := range value {
				resolved, isSecret, err := resolveValue(ctx, opts.Resolver, scope, item)
				if err != nil {
					return nil, fmt.Errorf("config: key %q[%d]: %w", key, i, err)
				}
				expanded, err := expandVars(resolved, opts.Vars)
				if err != nil {
					return nil, fmt.Errorf("config: key %q[%d]: %w", key, i, err)
				}
				if isSecret && !m.secret {
					m.secret = true
					m.ref = item
				}
				items[i] = expanded
			}
			cfg.put(key, items, m)
		default:
			return nil, fmt.Errorf("config: key %q: unsupported value type %T", key, value)
		}
	}
	return cfg, nil
}

// readDocument parses a JSON configuration file. The top level must be an
// object whose values are strings or arrays of strings.
func readDocument(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	doc := make(map[string]any, len(raw))
	for key, value := range raw {
		var s string
		if err := json.Unmarshal(value, &s); err == nil {
			doc[key] = s
			continue
		}
		var list []string
		if err := json.Unmarshal(value, &list); err == nil {
			doc[key] = list
			continue
		}
		return nil, fmt.Errorf("config: parse %s: key %q must be a string or a list of strings", path, key)
	}
	return doc, nil
}

// expandVars substitutes ${NAME} placeholders with the matching entry from
// vars. An unknown placeholder is an error.
func expandVars(s string, vars map[string]string) (string, error) {
	if !strings.Contains(s, "${") {
		return s, nil
	}
	var b strings.Builder
	for {
		start := strings.Index(s, "${")
		if start < 0 {
			b.WriteString(s)
			return b.String(), nil
		}
		b.WriteString(s[:start])
		rest := s[start+2:]
		end := strings.Index(rest, "}")
		if end < 0 {
			return "", fmt.Errorf("unterminated placeholder in %q", s)
		}
		name := rest[:end]
		value, ok := vars[name]
		if !ok {
			return "", fmt.Errorf("unknown placeholder ${%s}", name)
		}
		b.WriteString(value)
		s = rest[end+1:]
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
