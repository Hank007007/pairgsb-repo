package policy

import (
	"condgrate/internal/cond"
)

// Decision is the merged outcome of all candidates for one request.
type Decision struct {
	Effect      Effect
	Obligations []string
	Matched     []string
	Reasons     []string
}

// Combine merges the candidates selected by Collect into one decision.
// The first matched candidate in collection order determines the effect, so
// the order in which policies are listed decides the outcome when several
// candidates match. Obligations are gathered from the permit candidates.
func Combine(candidates []Candidate) Decision {
	matched := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		if c.Verdict == cond.True {
			matched = append(matched, c)
		}
	}
	if len(matched) == 0 {
		return Decision{Effect: Deny, Reasons: []string{"no policy matched"}}
	}

	winner := matched[0]
	decision := Decision{
		Effect:  winner.Policy.Effect,
		Reasons: []string{"decided by " + winner.Policy.ID},
	}
	for _, c := range candidates {
		if c.Policy.Effect == Permit {
			decision.Obligations = append(decision.Obligations, c.Policy.Obligations...)
		}
	}
	for _, c := range matched {
		decision.Matched = append(decision.Matched, c.Policy.ID)
	}
	return decision
}
