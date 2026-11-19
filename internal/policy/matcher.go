// Package policy loads accessor policies and decides whether an accessor may
// read a given secret path.
package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Effect is the outcome a rule asks for.
type Effect string

const (
	Allow Effect = "allow"
	Deny  Effect = "deny"
)

// Rule is one pattern/effect pair attached to an accessor.
type Rule struct {
	Effect Effect `json:"effect"`
	Path   string `json:"path"`
}

// Accessor is a caller identity together with its rules.
type Accessor struct {
	ID    string `json:"id"`
	Token string `json:"token"`
	Rules []Rule `json:"rules"`
}

// Policy is the loaded set of accessors, indexed by token.
type Policy struct {
	byToken map[string]*Accessor
}

type fileShape struct {
	Accessors []Accessor `json:"accessors"`
}

// MatchGlob reports whether path matches pattern. Both arguments are full
// paths; matching is anchored at both ends. A `*` segment matches exactly one
// non-empty segment, while a `**` segment matches zero or more segments.
func MatchGlob(pattern, path string) bool {
	patternSegs := strings.Split(pattern, "/")
	pathSegs := strings.Split(path, "/")

	// dp[i][j] reports whether patternSegs[i:] can match pathSegs[j:].
	dp := make([][]bool, len(patternSegs)+1)
	for i := range dp {
		dp[i] = make([]bool, len(pathSegs)+1)
	}
	dp[len(patternSegs)][len(pathSegs)] = true

	for i := len(patternSegs) - 1; i >= 0; i-- {
		for j := len(pathSegs); j >= 0; j-- {
			switch patternSegs[i] {
			case "**":
				// Match zero segments or consume one more segment.
				if dp[i+1][j] {
					dp[i][j] = true
				}
				if j < len(pathSegs) && pathSegs[j] != "" && dp[i][j+1] {
					dp[i][j] = true
				}
			case "*":
				if j < len(pathSegs) && pathSegs[j] != "" && dp[i+1][j+1] {
					dp[i][j] = true
				}
			default:
				if j < len(pathSegs) && pathSegs[j] == patternSegs[i] && dp[i+1][j+1] {
					dp[i][j] = true
				}
			}
		}
	}
	return dp[0][0]
}

// Load reads a policy file and returns its accessors.
func Load(path string) ([]Accessor, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read policy file: %w", err)
	}
	var doc fileShape
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse policy file: %w", err)
	}
	for i, a := range doc.Accessors {
		if a.ID == "" {
			return nil, fmt.Errorf("accessors[%d]: id is required", i)
		}
		if a.Token == "" {
			return nil, fmt.Errorf("accessors[%d] (%s): token is required", i, a.ID)
		}
		for j, r := range a.Rules {
			if r.Effect != Allow && r.Effect != Deny {
				return nil, fmt.Errorf("accessors[%d] (%s) rules[%d]: unknown effect %q", i, a.ID, j, r.Effect)
			}
			if r.Path == "" {
				return nil, fmt.Errorf("accessors[%d] (%s) rules[%d]: path is required", i, a.ID, j)
			}
		}
	}
	return doc.Accessors, nil
}

// New builds a Policy from accessors, rejecting duplicate ids and tokens.
func New(accessors []Accessor) (*Policy, error) {
	p := &Policy{byToken: make(map[string]*Accessor, len(accessors))}
	seenID := make(map[string]bool, len(accessors))
	for i := range accessors {
		a := accessors[i]
		if _, dup := p.byToken[a.Token]; dup {
			return nil, fmt.Errorf("duplicate accessor token for %q", a.ID)
		}
		if seenID[a.ID] {
			return nil, fmt.Errorf("duplicate accessor id %q", a.ID)
		}
		seenID[a.ID] = true
		copied := a
		p.byToken[a.Token] = &copied
	}
	return p, nil
}

// ByToken resolves an accessor token.
func (p *Policy) ByToken(token string) (*Accessor, bool) {
	a, ok := p.byToken[token]
	return a, ok
}

// Evaluate decides whether a is allowed to read path. Deny always wins over
// allow; rule order has no semantic effect.
func (a *Accessor) Evaluate(path string) bool {
	allowed := false
	for _, r := range a.Rules {
		if !MatchGlob(r.Path, path) {
			continue
		}
		if r.Effect == Deny {
			return false
		}
		if r.Effect == Allow {
			allowed = true
		}
	}
	return allowed
}
