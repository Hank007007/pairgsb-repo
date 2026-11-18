package policy

import (
	"strings"
	"time"
)

// Matches reports whether r applies to a request at time now.
//
// tenant, subject, action and resource are the request values. The rule is
// considered only when it is inside its validity interval.
func Matches(r Rule, tenant, subject, action, resource string, now time.Time) bool {
	if !tenantMatches(r.Tenant, tenant) {
		return false
	}
	if !matchSubject(r.Subject, subject) {
		return false
	}
	if !matchAction(r.Action, action) {
		return false
	}
	if !matchSubject(r.Resource, resource) {
		return false
	}
	return inWindow(r, now)
}

func inWindow(r Rule, now time.Time) bool {
	if !r.NotBefore.IsZero() && now.Before(r.NotBefore) {
		return false
	}
	if !r.NotAfter.IsZero() && !now.Before(r.NotAfter) {
		return false
	}
	return true
}

func tenantMatches(pattern, tenant string) bool {
	return pattern == "*" || pattern == tenant
}

// matchSubject implements the wildcard matching used for subject and resource
// patterns: '*' stands for any run of characters.
func matchSubject(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return pattern == value
	}

	// Classic wildcard match with backtracking. The last '*' seen is the
	// fallback point when the remainder of the pattern fails to line up.
	p, v := 0, 0
	star, match := -1, 0
	for v < len(value) {
		switch {
		case p < len(pattern) && pattern[p] == '*':
			star = p
			match = v
			p++
		case p < len(pattern) && pattern[p] == value[v]:
			p++
			v++
		case star >= 0:
			p = star + 1
			match++
			v = match
		default:
			return false
		}
	}
	for p < len(pattern) && pattern[p] == '*' {
		p++
	}
	return p == len(pattern)
}

// matchAction matches a dotted action identifier against a pattern.
func matchAction(pattern, action string) bool {
	if pattern == action {
		return true
	}
	patternParts := strings.Split(pattern, ".")
	actionParts := strings.Split(action, ".")
	if len(patternParts) != len(actionParts) {
		return false
	}
	for i, part := range patternParts {
		if part == "*" {
			continue
		}
		if part != actionParts[i] {
			return false
		}
	}
	return true
}

// Evaluate returns the combined effect of every rule that matches the request.
// Deny takes precedence over allow. The second result is the ID of the rule
// that determined the outcome, and is empty when the default decision applies.
func Evaluate(b *Bundle, tenant, subject, action, resource string, now time.Time) (Effect, string) {
	effect := Allow
	var matched string
	for _, r := range b.Rules {
		if !Matches(r, tenant, subject, action, resource, now) {
			continue
		}
		if r.Effect == Deny {
			return Deny, r.ID
		}
		effect = Allow
		if matched == "" {
			matched = r.ID
		}
	}
	return effect, matched
}

// NextBoundary returns the earliest time after now at which some rule starts
// or stops being valid. The boolean is false when no rule has a finite future
// boundary.
func NextBoundary(rules []Rule, now time.Time) (time.Time, bool) {
	var next time.Time
	found := false
	consider := func(t time.Time) {
		if t.IsZero() || !t.After(now) {
			return
		}
		if !found || t.Before(next) {
			next = t
			found = true
		}
	}
	for _, r := range rules {
		consider(r.NotBefore)
		consider(r.NotAfter)
	}
	return next, found
}
