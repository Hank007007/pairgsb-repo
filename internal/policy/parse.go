// Package policy loads redaction rules and matches field paths against them.
package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Action is what the journal does with a field that matches a rule.
type Action string

const (
	// ActionRedact replaces the field value with a commitment.
	ActionRedact Action = "redact"
	// ActionKeep writes the field value verbatim.
	ActionKeep Action = "keep"
)

// defaultAction is used for fields that no rule matches.
const defaultAction = ActionKeep

// Rule is one compiled redaction rule.
type Rule struct {
	// Path is the rule pattern in the redaction path grammar, for example
	// "user.password" or "items[].ssn".
	Path string
	// Action is applied to every field whose path the rule matches.
	Action Action
}

// RuleSet is an ordered list of rules. The first matching rule wins.
type RuleSet struct {
	Rules []Rule
}

// ruleFile is the on-disk policy document.
type ruleFile struct {
	Rules []struct {
		Path   string `json:"path"`
		Action string `json:"action"`
	} `json:"rules"`
}

// Parse decodes a policy document.
func Parse(data []byte) (*RuleSet, error) {
	var doc ruleFile
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("policy: invalid document: %w", err)
	}
	rs := &RuleSet{}
	for i, raw := range doc.Rules {
		path := strings.TrimSpace(raw.Path)
		if path == "" {
			return nil, fmt.Errorf("policy: rule %d has an empty path", i)
		}
		action := Action(strings.ToLower(strings.TrimSpace(raw.Action)))
		if action == "" {
			action = ActionRedact
		}
		switch action {
		case ActionRedact, ActionKeep:
		default:
			return nil, fmt.Errorf("policy: rule %d has unknown action %q", i, raw.Action)
		}
		rs.Rules = append(rs.Rules, Rule{Path: path, Action: action})
	}
	return rs, nil
}

// Load reads and parses a policy file.
func Load(path string) (*RuleSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("policy: %w", err)
	}
	return Parse(data)
}

// Empty reports whether the policy contains no rules.
func (rs *RuleSet) Empty() bool {
	return rs == nil || len(rs.Rules) == 0
}
