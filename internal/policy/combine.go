package policy

import (
	"strings"

	"condgrate/internal/cond"
)

// Decision is the merged outcome of all candidates for one request.
type Decision struct {
	Effect      Effect
	Obligations []string
	Matched     []string
	Reasons     []string
}

// Combine merges the candidates selected by Collect into one decision. Deny
// wins over permit regardless of layer or collection order, and obligations
// are only returned with a permit decision.
func Combine(candidates []Candidate) Decision {
	matched := make([]Candidate, 0, len(candidates))
	indeterminate := make([]string, 0)
	for _, c := range candidates {
		switch c.Verdict {
		case cond.True:
			matched = append(matched, c)
		case cond.Indeterminate:
			indeterminate = append(indeterminate, c.Policy.ID)
		}
	}
	if len(indeterminate) > 0 {
		ids := make([]string, 0, len(matched))
		for _, c := range matched {
			ids = append(ids, c.Policy.ID)
		}
		return Decision{
			Effect:  Deny,
			Matched: ids,
			Reasons: []string{"condition indeterminate in " + strings.Join(indeterminate, ", ")},
		}
	}
	if len(matched) == 0 {
		return Decision{Effect: Deny, Reasons: []string{"no policy matched"}}
	}

	ids := make([]string, 0, len(matched))
	denies := make([]string, 0, len(matched))
	permits := make([]string, 0, len(matched))
	for _, c := range matched {
		ids = append(ids, c.Policy.ID)
		switch c.Policy.Effect {
		case Deny:
			denies = append(denies, c.Policy.ID)
		case Permit:
			permits = append(permits, c.Policy.ID)
		}
	}

	if len(denies) > 0 {
		return Decision{
			Effect:  Deny,
			Matched: ids,
			Reasons: []string{"decided by " + strings.Join(denies, ", ")},
		}
	}

	decision := Decision{
		Effect:  Permit,
		Matched: ids,
		Reasons: []string{"decided by " + strings.Join(permits, ", ")},
	}
	seen := make(map[string]bool)
	for _, c := range matched {
		if c.Policy.Effect != Permit || seen[c.Policy.ID] {
			continue
		}
		seen[c.Policy.ID] = true
		for _, obligation := range c.Policy.Obligations {
			decision.Obligations = append(decision.Obligations, obligation)
		}
	}
	return decision
}
