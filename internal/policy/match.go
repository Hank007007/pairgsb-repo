// Package policy loads the allow-list and decides whether a name is allowed.
package policy

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Rule is one entry of the allow-list.
type Rule struct {
	// Wildcard is set for rules written as "*." followed by a suffix.
	Wildcard bool
	// Name is the normalized name, without the leading "*." for wildcards.
	Name string
}

// Errors reported by this package.
var (
	ErrBadRule = errors.New("policy: malformed rule")
)

// LoadRules reads an allow-list in the format of docs/policy.md §2.
func LoadRules(r io.Reader) ([]Rule, error) {
	var rules []Rule
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		rule, err := parseRule(text)
		if err != nil {
			return nil, fmt.Errorf("%w on line %d", err, line)
		}
		rules = append(rules, rule)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return rules, nil
}

func parseRule(text string) (Rule, error) {
	if strings.HasPrefix(text, "*.") {
		suffix := text[2:]
		if suffix == "" || strings.Contains(suffix, "*") {
			return Rule{}, ErrBadRule
		}
		return Rule{Wildcard: true, Name: normalizeHost(suffix)}, nil
	}
	if text == "" || strings.Contains(text, "*") {
		return Rule{}, ErrBadRule
	}
	return Rule{Name: normalizeHost(text)}, nil
}

// Match reports whether name is allowed by rules. It returns the matching rule
// so callers can log the decision.
func Match(rules []Rule, name string) (Rule, bool) {
	for _, rule := range rules {
		if rule.Wildcard {
			if strings.HasSuffix(name, rule.Name) {
				return rule, true
			}
			continue
		}
		if name == rule.Name {
			return rule, true
		}
	}
	return Rule{}, false
}

// normalizeHost prepares a name for comparison.
func normalizeHost(name string) string {
	name = strings.TrimSuffix(name, ".")
	return strings.ToLower(name)
}
