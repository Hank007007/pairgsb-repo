package acl

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

// Operation names accepted in a rule. Deny is the special op that refuses
// everything below the pattern it is attached to.
const (
	OpRead  = "read"
	OpWrite = "write"
	OpList  = "list"
	OpChown = "chown"
	OpDeny  = "deny"
)

var knownOps = map[string]bool{
	OpRead:  true,
	OpWrite: true,
	OpList:  true,
	OpChown: true,
	OpDeny:  true,
}

// Rule is one entry of the access file.
type Rule struct {
	UID   int      `json:"uid"`
	Paths []string `json:"paths"`
	Ops   []string `json:"ops"`
}

// Allows reports whether the rule grants op.
func (r *Rule) Allows(op string) bool {
	for _, have := range r.Ops {
		if have == op {
			return true
		}
	}
	return false
}

// Denies reports whether the rule is a deny rule.
func (r *Rule) Denies() bool {
	return r.Allows(OpDeny)
}

type accessFile struct {
	Rules []*Rule `json:"rules"`
}

// Policy is a loaded access file, indexed by caller uid and by pattern.
type Policy struct {
	byUID map[int]map[string]*Rule
	rules []*Rule
}

// Parse reads an access file from r.
func Parse(r io.Reader) (*Policy, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()

	var doc accessFile
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("acl: decode access file: %w", err)
	}
	if err := dec.Decode(new(struct{})); err != io.EOF {
		return nil, fmt.Errorf("acl: trailing data in access file")
	}
	return newPolicy(doc.Rules)
}

// LoadFile reads an access file from disk.
func LoadFile(name string) (*Policy, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	p, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return p, nil
}

func newPolicy(rules []*Rule) (*Policy, error) {
	p := &Policy{
		byUID: make(map[int]map[string]*Rule),
		rules: make([]*Rule, 0, len(rules)),
	}
	for i, rule := range rules {
		if rule == nil {
			return nil, fmt.Errorf("acl: rule %d is null", i)
		}
		if rule.UID < 0 {
			return nil, fmt.Errorf("acl: rule %d has a negative uid", i)
		}
		if len(rule.Paths) == 0 {
			return nil, fmt.Errorf("acl: rule %d for uid %d has no paths", i, rule.UID)
		}
		if len(rule.Ops) == 0 {
			return nil, fmt.Errorf("acl: rule %d for uid %d has no ops", i, rule.UID)
		}
		for _, op := range rule.Ops {
			if !knownOps[op] {
				return nil, fmt.Errorf("acl: rule %d for uid %d has unknown op %q", i, rule.UID, op)
			}
		}
		if rule.Denies() && len(rule.Ops) != 1 {
			return nil, fmt.Errorf("acl: rule %d for uid %d mixes deny with other ops", i, rule.UID)
		}

		patterns := p.byUID[rule.UID]
		if patterns == nil {
			patterns = make(map[string]*Rule)
			p.byUID[rule.UID] = patterns
		}
		for _, pattern := range rule.Paths {
			if err := ValidPattern(pattern); err != nil {
				return nil, fmt.Errorf("acl: rule %d for uid %d: %q: %w", i, rule.UID, pattern, err)
			}
			patterns[pattern] = rule
		}
		p.rules = append(p.rules, rule)
	}
	return p, nil
}

// Rules returns the rules of uid in the order they were declared.
func (p *Policy) Rules(uid int) []*Rule {
	out := make([]*Rule, 0)
	for _, rule := range p.rules {
		if rule.UID == uid {
			out = append(out, rule)
		}
	}
	return out
}

// UIDs returns every uid that has at least one rule, in ascending order.
func (p *Policy) UIDs() []int {
	out := make([]int, 0, len(p.byUID))
	for uid := range p.byUID {
		out = append(out, uid)
	}
	sort.Ints(out)
	return out
}
