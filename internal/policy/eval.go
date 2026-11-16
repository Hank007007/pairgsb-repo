package policy

import "fmt"

// SourcePolicy marks a decision produced by fresh evaluation.
const SourcePolicy = "policy"

// SourceCache marks a decision served from the decision cache.
const SourceCache = "cache"

// ReasonNoMatch is the reason recorded when nothing in the policy speaks to a request.
const ReasonNoMatch = "no matching rule"

// Engine evaluates requests against a single policy document.
type Engine struct {
	policy *Policy
}

// NewEngine builds an engine for a policy document.
func NewEngine(p *Policy) *Engine {
	if p == nil {
		p = &Policy{}
	}
	return &Engine{policy: p}
}

// Policy returns the document the engine evaluates.
func (e *Engine) Policy() *Policy {
	return e.policy
}

// Evaluate answers a request against the policy rules, expanding delegation first so that
// inherited grants take part in the same pass as directly written rules.
func (e *Engine) Evaluate(req Request) Decision {
	resource := NormalizePath(req.Resource)
	effective := e.policy.ExpandDelegation(req.Subject.Roles)

	for _, rule := range effective {
		if !req.Subject.hasRole(rule.Role) {
			continue
		}
		if !MatchResource(rule.Resource, resource) {
			continue
		}
		if !actionCovered(rule.Actions, req.Action) {
			continue
		}
		return Decision{
			Effect: rule.Effect,
			Reason: fmt.Sprintf("rule %q applies to %s on %s", rule.Name, req.Action, resource),
			Source: SourcePolicy,
		}
	}

	return Decision{
		Effect: Deny,
		Reason: ReasonNoMatch,
		Source: SourcePolicy,
	}
}
