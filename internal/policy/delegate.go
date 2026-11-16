package policy

import "fmt"

// EffectiveRule is a rule in force for a particular role, either written directly in the
// policy for that role or reached through a delegation edge.
type EffectiveRule struct {
	Role     string
	Name     string
	Resource string
	Actions  []string
	Effect   Effect
}

// ExpandDelegation returns the rules the given role chain can exercise, combining the rules
// written directly for those roles with the grants reached along delegation edges.
func (p *Policy) ExpandDelegation(roles []string) []EffectiveRule {
	effective := make([]EffectiveRule, 0, len(p.Rules))
	for _, role := range roles {
		for _, rule := range p.Rules {
			if !roleIn(rule.Roles, role) {
				continue
			}
			effective = append(effective, EffectiveRule{
				Role:     role,
				Name:     rule.Name,
				Resource: rule.Resource,
				Actions:  rule.Actions,
				Effect:   rule.Effect,
			})
		}
	}
	for _, role := range roles {
		effective = append(effective, p.expandRole(role, 1)...)
	}
	return effective
}

// expandRole follows the delegation edges arriving at role while budget remains; each edge
// traversed spends one unit of budget. Grants found this way are exercised under role.
func (p *Policy) expandRole(role string, budget int) []EffectiveRule {
	if budget <= 0 {
		return nil
	}
	seen := make(map[string]bool)
	seen[role] = true

	var out []EffectiveRule
	for _, edge := range p.Delegations {
		if edge.To != role || seen[edge.From] {
			continue
		}
		out = append(out, EffectiveRule{
			Role:     role,
			Name:     fmt.Sprintf("delegation:%s->%s", edge.From, edge.To),
			Resource: edge.Resource,
			Actions:  edge.Actions,
			Effect:   Allow,
		})
		out = append(out, p.expandRole(edge.From, budget-1)...)
	}
	return out
}

// roleIn reports whether roles contains name.
func roleIn(roles []string, name string) bool {
	for _, r := range roles {
		if r == name {
			return true
		}
	}
	return false
}
