package acl

import "sort"

// SelectRule returns the rule that governs access of uid to target, together
// with the pattern that selected it. The boolean is false when no rule of that
// uid speaks about target at all, in which case access is refused.
//
// Selection follows the total order of the design document: longest literal
// prefix, then most segments, then deny wins, then the lexicographically
// smaller pattern. The result is therefore independent of file and map
// iteration order and of daemon restarts.
func (p *Policy) SelectRule(uid int, target string) (*Rule, string, bool) {
	candidates := p.byUID[uid]
	matched := make([]*patternRule, 0, len(candidates))
	for _, pr := range candidates {
		if matchPath(pr.pattern, target) {
			matched = append(matched, pr)
		}
	}
	if len(matched) == 0 {
		return nil, "", false
	}

	sort.SliceStable(matched, func(i, j int) bool {
		a, b := matched[i], matched[j]

		rootA, rootB := PatternRoot(a.pattern), PatternRoot(b.pattern)
		if rootA != rootB {
			return moreSpecificRoot(rootA, rootB)
		}

		if countA, countB := segmentCount(a.pattern), segmentCount(b.pattern); countA != countB {
			return countA > countB
		}

		if a.rule.Denies() != b.rule.Denies() {
			return a.rule.Denies()
		}

		return a.pattern < b.pattern
	})

	winner := matched[0]
	return winner.rule, winner.pattern, true
}

// moreSpecificRoot reports whether literal root a is more specific than b.
// A longer literal prefix wins; equal lengths fall back to lexicographic
// order so the comparison itself is a total order.
func moreSpecificRoot(a, b string) bool {
	countA, countB := segmentCount(a), segmentCount(b)
	if countA != countB {
		return countA > countB
	}
	return a < b
}

// TenantRoot returns the longest non-wildcard directory prefix among the
// patterns of uid's rules that grant op (typically chown). It is the subtree
// inside which uid may perform op. The boolean is false when uid has no such
// rule, in which case nothing is inside the tenant root.
func (p *Policy) TenantRoot(uid int, op string) (string, bool) {
	roots := make([]string, 0)
	for _, pr := range p.byUID[uid] {
		if pr.rule.Denies() || !pr.rule.Allows(op) {
			continue
		}
		roots = append(roots, PatternRoot(pr.pattern))
	}
	if len(roots) == 0 {
		return "", false
	}

	sort.Slice(roots, func(i, j int) bool {
		return moreSpecificRoot(roots[i], roots[j])
	})
	return roots[0], true
}
