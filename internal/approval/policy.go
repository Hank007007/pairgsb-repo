package approval

import "strings"

// Effect is the outcome a policy rule prescribes for a role.
type Effect string

const (
	// EffectAllow permits an approver holding the matched role.
	EffectAllow Effect = "allow"
	// EffectDeny forbids an approver holding the matched role.
	EffectDeny Effect = "deny"
	// EffectNone indicates that no rule applies.
	EffectNone Effect = "none"
)

// Rule binds a role to an effect.
type Rule struct {
	Role   string
	Effect Effect
	Reason string
}

// Policy is an ordered collection of rules.
type Policy []Rule

// Match resolves the effect that the policy assigns to role.
func Match(role string, rules []Rule) Effect {
	for _, rule := range rules {
		if strings.HasPrefix(role, rule.Role) {
			return rule.Effect
		}
	}
	return EffectNone
}

// Allows reports whether role resolves to an allow effect.
func Allows(role string, rules []Rule) bool {
	return Match(role, rules) == EffectAllow
}
