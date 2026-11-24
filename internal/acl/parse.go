// Package acl loads the authorization rule set and answers whether a topic
// matches a filter.
package acl

import (
	"fmt"
	"sync"

	"gopkg.in/yaml.v3"
)

// Effect is the outcome a rule asks for.
type Effect string

const (
	// EffectAllow permits the operation when the rule is the effective one.
	EffectAllow Effect = "allow"
	// EffectDeny forbids the operation when the rule is the effective one.
	EffectDeny Effect = "deny"
)

// Rule is one entry of the policy document.
type Rule struct {
	ID        string `json:"id"`
	Effect    Effect `json:"effect"`
	Principal string `json:"principal"`
	Action    string `json:"action"`
	Filter    string `json:"filter"`
	Tenant    string `json:"tenant,omitempty"`
}

// RuleSet is a loaded policy document. A RuleSet is never modified after it
// has been published; reloads install a new value.
type RuleSet struct {
	Version uint64 `json:"version"`
	Rules   []Rule `json:"rules"`
}

// rawRule mirrors the YAML layout of one rule.
type rawRule struct {
	ID        string `yaml:"id"`
	Effect    string `yaml:"effect"`
	Principal string `yaml:"principal"`
	Action    string `yaml:"action"`
	Filter    string `yaml:"filter"`
	Tenant    string `yaml:"tenant"`
}

// LoadRules parses a policy document. The returned RuleSet has version zero;
// the Store assigns the version when it installs the document.
func LoadRules(data []byte) (*RuleSet, error) {
	var doc struct {
		Rules []rawRule `yaml:"rules"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("acl: parse rules: %w", err)
	}

	rs := &RuleSet{Rules: make([]Rule, 0, len(doc.Rules))}
	seen := make(map[string]struct{}, len(doc.Rules))
	for i, raw := range doc.Rules {
		if raw.ID == "" {
			return nil, fmt.Errorf("acl: rule %d: missing id", i)
		}
		if _, dup := seen[raw.ID]; dup {
			return nil, fmt.Errorf("acl: duplicate rule id %q", raw.ID)
		}
		seen[raw.ID] = struct{}{}

		effect, err := parseEffect(raw.Effect)
		if err != nil {
			return nil, fmt.Errorf("acl: rule %d (%q): %w", i, raw.ID, err)
		}

		rs.Rules = append(rs.Rules, Rule{
			ID:        raw.ID,
			Effect:    effect,
			Principal: raw.Principal,
			Action:    raw.Action,
			Filter:    raw.Filter,
			Tenant:    raw.Tenant,
		})
	}
	return rs, nil
}

// parseEffect maps the textual effect of a rule onto an Effect. Only the two
// documented spellings are accepted.
func parseEffect(raw string) (Effect, error) {
	switch Effect(raw) {
	case EffectAllow:
		return EffectAllow, nil
	case EffectDeny:
		return EffectDeny, nil
	default:
		return "", fmt.Errorf("invalid effect %q: must be %q or %q", raw, EffectAllow, EffectDeny)
	}
}

// ParseEffect validates and maps a textual effect.
func ParseEffect(raw string) (Effect, error) {
	return parseEffect(raw)
}

// Store holds the active rule set and swaps it on reload.
type Store struct {
	mu    sync.RWMutex
	rules *RuleSet
}

// NewStore returns a Store holding rs. A nil rs means an empty rule set.
func NewStore(rs *RuleSet) *Store {
	if rs == nil {
		rs = &RuleSet{Rules: nil}
	}
	return &Store{rules: rs}
}

// Current returns the active rule set. The returned value is read-only.
func (s *Store) Current() *RuleSet {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rules
}

// Replace loads data and installs it as the new active rule set, advancing the
// version. When loading fails the active rule set is left untouched.
func (s *Store) Replace(data []byte) error {
	next, err := LoadRules(data)
	if err != nil {
		return err
	}

	s.mu.Lock()
	currentVersion := uint64(0)
	if s.rules != nil {
		currentVersion = s.rules.Version
	}
	next.Version = currentVersion + 1
	s.rules = next
	s.mu.Unlock()
	return nil
}

// Version reports the version of the active rule set.
func (s *Store) Version() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rules.Version
}
