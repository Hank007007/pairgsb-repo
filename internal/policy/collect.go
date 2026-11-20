package policy

import (
	"fmt"
	"strings"

	"condgrate/internal/cond"
)

// Candidate is one policy selected for a decision, together with the
// condition result computed for it.
type Candidate struct {
	Policy  Policy
	Layer   Layer
	Verdict cond.Result
}

// MatchRequest is the input to target and policy matching.
type MatchRequest struct {
	Principal MatchBlock
	Resource  MatchBlock
	Action    string
}

// MatchesAttr reports whether value is accepted by any candidate pattern.
func MatchesAttr(patterns []string, value string) bool {
	for _, p := range patterns {
		if matchPattern(p, value) {
			return true
		}
	}
	return false
}

func matchPattern(pattern, value string) bool {
	patternParts := strings.Split(pattern, ":")
	valueParts := strings.Split(value, ":")
	if len(patternParts) != len(valueParts) {
		return false
	}
	for i := range patternParts {
		if patternParts[i] != "*" && patternParts[i] != valueParts[i] {
			return false
		}
	}
	return true
}

func matchesBlock(block MatchBlock, values MatchBlock) bool {
	for attr, patterns := range block {
		actual, ok := values[attr]
		if !ok {
			return false
		}
		matched := false
		for _, candidate := range actual {
			if MatchesAttr(patterns, candidate) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func matchesAction(patterns []string, action string) bool {
	if len(patterns) == 0 {
		return true
	}
	return MatchesAttr(patterns, action)
}

// targetMatches reports whether req satisfies the layer target.
func targetMatches(t Target, req MatchRequest) bool {
	if !matchesBlock(t.Principal, req.Principal) {
		return false
	}
	if !matchesBlock(t.Resource, req.Resource) {
		return false
	}
	return matchesAction(t.Action, req.Action)
}

// policyMatches reports whether req satisfies a policy's match blocks.
func policyMatches(p Policy, req MatchRequest) bool {
	if !matchesBlock(p.Principal, req.Principal) {
		return false
	}
	if !matchesBlock(p.Resource, req.Resource) {
		return false
	}
	return matchesAction(p.Action, req.Action)
}

// Scopes returns the chain ordered from the most specific layer upward.
func (c *Chain) Scopes() []Scope {
	return []Scope{c.Project, c.Team, c.Org}
}

// Collect gathers every policy on the chain whose layer target and own match
// blocks accept the request. Each layer is visited from the most specific one
// upward, and every matching policy on every layer is retained.
func Collect(c *Chain, req MatchRequest, env cond.Env) []Candidate {
	var out []Candidate
	for _, scope := range c.Scopes() {
		if !targetMatches(scope.Target, req) {
			continue
		}
		for _, p := range scope.Policies {
			if !policyMatches(p, req) {
				continue
			}
			out = append(out, Candidate{
				Policy:  p,
				Layer:   scope.Layer,
				Verdict: cond.Eval(p.Condition, env),
			})
		}
	}
	return out
}

// Describe renders a candidate for diagnostics.
func (c Candidate) Describe() string {
	return fmt.Sprintf("%s/%s/%s", c.Layer, c.Policy.ID, c.Verdict)
}
