package approval

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

// roleParents maps each role to its parent in the hierarchy defined by
// §6.1 (child -> parent). Roles absent from the map are hierarchy roots.
var roleParents = map[string]string{
	"ops":          "admin",
	"auditor":      "admin",
	"ops-oncall":   "ops",
	"ops-readonly": "auditor",
}

// isOrAncestor reports whether ruleRole is the requested role or one of its
// ancestors reachable by walking the parent chain upward. Matching is purely
// hierarchical; string prefixes, suffixes and substring tests are not used.
func isOrAncestor(ruleRole, requestedRole string) bool {
	for role := requestedRole; ; {
		if role == ruleRole {
			return true
		}
		parent, ok := roleParents[role]
		if !ok {
			return false
		}
		role = parent
	}
}

// Policy is an ordered collection of rules.
type Policy []Rule

// Match resolves the effect that the policy assigns to role. Every
// applicable rule is collected regardless of ordering: any matching deny
// yields deny, otherwise any matching allow yields allow, otherwise none.
func Match(role string, rules []Rule) Effect {
	hasAllow := false
	for _, rule := range rules {
		if !isOrAncestor(rule.Role, role) {
			continue
		}
		switch rule.Effect {
		case EffectDeny:
			return EffectDeny
		case EffectAllow:
			hasAllow = true
		}
	}
	if hasAllow {
		return EffectAllow
	}
	return EffectNone
}

// Allows reports whether role resolves to an allow effect.
func Allows(role string, rules []Rule) bool {
	return Match(role, rules) == EffectAllow
}
