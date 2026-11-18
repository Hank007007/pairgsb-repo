package policy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Errors returned by authorization checks.
var (
	ErrDenied  = errors.New("policy: call denied")
	ErrNoMatch = errors.New("policy: no rule matched")
)

// PolicySet is the merged set of call tables loaded from a policy directory.
type PolicySet struct {
	// Name identifies the set in audit records.
	Name string
	// Version is the highest document version that contributed to the set.
	Version int
	// Rules holds every rule in the set, keyed by its qualified rule ID.
	Rules map[string]Rule
}

// Load reads the trust configuration and every call table in dir.
func Load(dir string) (*PolicySet, error) {
	if _, err := LoadTrust(filepath.Join(dir, "trust.json")); err != nil {
		return nil, err
	}

	files, err := policyFiles(dir)
	if err != nil {
		return nil, fmt.Errorf("policy: read dir: %w", err)
	}

	set := &PolicySet{
		Name:  "call-tables",
		Rules: make(map[string]Rule),
	}
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("policy: read %s: %w", path, err)
		}
		doc, err := parseDocument(raw, filepath.Base(path))
		if err != nil {
			return nil, err
		}
		if doc.Version > set.Version {
			set.Version = doc.Version
		}
		for _, rule := range doc.Rules {
			key := rule.ID
			if key == "" {
				key = fmt.Sprintf("%s#%s>%s", doc.Name, rule.From, rule.To)
			}
			if _, dup := set.Rules[key]; dup {
				key = doc.Name + "/" + key
			}
			set.Rules[key] = rule
		}
	}

	if len(set.Rules) == 0 {
		return nil, fmt.Errorf("%w: no rules loaded from %s", ErrBadDocument, dir)
	}
	return set, nil
}

// Len reports how many rules the set holds.
func (s *PolicySet) Len() int {
	if s == nil {
		return 0
	}
	return len(s.Rules)
}

// Check decides whether the peer service may call method.
//
// A nil error means the call is allowed. Any non-nil error means it is denied.
func (s *PolicySet) Check(peer, method string) error {
	if s == nil || len(s.Rules) == 0 {
		return ErrNoMatch
	}

	for _, rule := range s.Rules {
		if !Allow(peer, method, rule) {
			continue
		}
		if rule.Effect == EffectPermit {
			return nil
		}
		return ErrDenied
	}
	return ErrDenied
}

// RulesFor returns the rules that name the given peer, in no particular order.
func (s *PolicySet) RulesFor(peer string) []Rule {
	if s == nil {
		return nil
	}
	var out []Rule
	for _, rule := range s.Rules {
		if rule.From == peer {
			out = append(out, rule)
		}
	}
	return out
}
