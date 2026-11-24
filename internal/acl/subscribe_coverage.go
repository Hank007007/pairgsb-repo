package acl

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"topicwarden/internal/tenant"
)

// CheckSubscription decides whether every concrete topic represented by a
// valid subscription filter is allowed. It returns the default-deny verdict
// when an expansion topic is uncovered and the deterministic deny verdict when
// a deny rule intersects the expansion.
func CheckSubscription(rs *RuleSet, principalClass, filter string) Verdict {
	_, verdict, _ := subscriptionCheck(rs, principalClass, filter)
	return verdict
}

func subscriptionCheck(rs *RuleSet, principalClass, filter string) (string, Verdict, bool) {
	normalized := tenant.NormalizeTopic(filter)
	levels := strings.Split(normalized, "/")
	if normalized == "" || !validFilterLevels(levels) {
		return "", Verdict{Allowed: false, Reason: "invalid topic filter"}, true
	}

	witness, verdict, found := subscriptionCounterexample(rs, principalClass, levels)
	if !found {
		return "", Verdict{Allowed: true}, false
	}
	if verdict.Reason == "" && !verdict.Allowed {
		verdict.Reason = "no matching rule"
	}
	return witness, verdict, true
}

func subscriptionCounterexample(rs *RuleSet, principalClass string, filter []string) (string, Verdict, bool) {
	if rs == nil {
		topic := firstExpansionWitness(filter)
		return topic, Evaluate(nil, principalClass, ActionSubscribe, topic), true
	}

	for _, rule := range rs.Rules {
		if rule.Principal != principalClass || rule.Action != ActionSubscribe || rule.Effect != EffectDeny {
			continue
		}
		ruleFilter := strings.Split(tenant.NormalizeTopic(rule.Filter), "/")
		if !validFilterLevels(ruleFilter) {
			continue
		}
		witness, ok := commonTopicWitness(
			filter,
			ruleFilter,
			tenantPatternLevels(rule.Tenant),
		)
		if !ok || witness == "" {
			continue
		}
		return witness, Evaluate(rs, principalClass, ActionSubscribe, witness), true
	}

	allows := make([]coverageMatcher, 0)
	for _, rule := range rs.Rules {
		if rule.Principal != principalClass || rule.Action != ActionSubscribe || rule.Effect != EffectAllow {
			continue
		}
		ruleFilter := strings.Split(tenant.NormalizeTopic(rule.Filter), "/")
		if !validFilterLevels(ruleFilter) {
			continue
		}
		allows = append(allows, coverageMatcher{
			ruleFilter:   ruleFilter,
			tenantFilter: tenantPatternLevels(rule.Tenant),
		})
	}

	initialRules := make([]int, len(allows))
	initialTenants := make([]int, len(allows))
	search := &coverageSearch{filter: filter, allows: allows, visited: make(map[string]struct{})}
	if witness := search.findUncovered(0, initialRules, initialTenants, nil); witness != "" {
		return witness, Evaluate(rs, principalClass, ActionSubscribe, witness), true
	}

	return "", Verdict{Allowed: true}, false
}

type coverageMatcher struct {
	ruleFilter   []string
	tenantFilter []string
}

type coverageSearch struct {
	filter  []string
	allows  []coverageMatcher
	visited map[string]struct{}
}

func firstExpansionWitness(filter []string) string {
	for i, level := range filter {
		if level == "#" {
			return strings.Join(append(append([]string{}, filter[:i]...), probeLevel), "/")
		}
		if level == "+" {
			filter[i] = probeLevel
		}
	}
	return strings.Join(filter, "/")
}

func tenantPatternLevels(tenantID string) []string {
	if tenantID == "" {
		return []string{"#"}
	}
	return []string{tenant.Prefix, tenantID, "#"}
}

func commonTopicWitness(patterns ...[]string) (string, bool) {
	indices := make([]int, len(patterns))
	var acc []string
	maxSteps := 1
	for _, pattern := range patterns {
		maxSteps += len(pattern) + 1
	}

	for {
		finished := true
		for i, pattern := range patterns {
			if !matcherCanFinish(pattern, indices[i]) {
				finished = false
				break
			}
		}
		if finished && strings.Join(acc, "/") != "" {
			return strings.Join(acc, "/"), true
		}

		forced := ""
		haveForced := false
		for i, pattern := range patterns {
			idx := indices[i]
			if idx >= len(pattern) {
				return "", false
			}
			if idx == len(pattern)-1 && pattern[idx] == "#" {
				continue
			}
			if pattern[idx] == "+" {
				continue
			}
			if haveForced && forced != pattern[idx] {
				return "", false
			}
			forced = pattern[idx]
			haveForced = true
		}

		level := probeLevel
		if haveForced {
			level = forced
		}

		nextIndices := make([]int, len(patterns))
		for i, pattern := range patterns {
			next := advanceMatcher(pattern, indices[i], level)
			if next < 0 {
				return "", false
			}
			nextIndices[i] = next
		}
		indices = nextIndices
		acc = append(acc, level)

		if len(acc) > maxSteps {
			return "", false
		}
	}
}

func (s *coverageSearch) findUncovered(filterIdx int, ruleIdx, tenantIdx []int, acc []string) string {
	n := len(s.allows)
	if s.filterCanFinish(filterIdx) && strings.Join(acc, "/") != "" {
		covered := false
		invincible := false
		for i := 0; i < n; i++ {
			ruleOK := matcherCanFinish(s.allows[i].ruleFilter, ruleIdx[i])
			tenantOK := matcherCanFinish(s.allows[i].tenantFilter, tenantIdx[i])
			if ruleOK && tenantOK {
				covered = true
				if matcherIsTerminalHash(s.allows[i].ruleFilter, ruleIdx[i]) &&
					matcherIsTerminalHash(s.allows[i].tenantFilter, tenantIdx[i]) {
					invincible = true
				}
			}
		}
		if !covered {
			return strings.Join(acc, "/")
		}
		if invincible {
			return ""
		}
	}

	if filterIdx >= len(s.filter) {
		return ""
	}

	stateKey := coverageStateKey(filterIdx, ruleIdx, tenantIdx)
	if _, seen := s.visited[stateKey]; seen {
		return ""
	}
	s.visited[stateKey] = struct{}{}

	candidates := s.candidateLevels(filterIdx, ruleIdx, tenantIdx)
	for _, level := range candidates {
		nextFilterIdx, ok := s.advanceFilter(filterIdx, level)
		if !ok {
			continue
		}

		nextRules := make([]int, n)
		nextTenants := make([]int, n)
		for i := 0; i < n; i++ {
			nextRules[i] = advanceMatcher(s.allows[i].ruleFilter, ruleIdx[i], level)
			nextTenants[i] = advanceMatcher(s.allows[i].tenantFilter, tenantIdx[i], level)
		}

		nextAcc := make([]string, len(acc)+1)
		copy(nextAcc, acc)
		nextAcc[len(acc)] = level

		if witness := s.findUncovered(nextFilterIdx, nextRules, nextTenants, nextAcc); witness != "" {
			return witness
		}
	}
	return ""
}

func (s *coverageSearch) filterCanFinish(idx int) bool {
	if idx == len(s.filter) {
		return true
	}
	return idx == len(s.filter)-1 && s.filter[idx] == "#"
}

func (s *coverageSearch) advanceFilter(idx int, level string) (int, bool) {
	if idx < 0 || idx >= len(s.filter) {
		return 0, false
	}
	switch s.filter[idx] {
	case "#":
		return idx, true
	case "+":
		if level == "" {
			return 0, false
		}
		return idx + 1, true
	default:
		if s.filter[idx] != level {
			return 0, false
		}
		return idx + 1, true
	}
}

func (s *coverageSearch) candidateLevels(filterIdx int, ruleIdx, tenantIdx []int) []string {
	if filterIdx >= len(s.filter) {
		return nil
	}
	if literal := s.filter[filterIdx]; literal != "+" && literal != "#" {
		return []string{literal}
	}

	relevant := make(map[string]struct{})
	includeEmpty := s.filter[filterIdx] == "#"
	for i := range s.allows {
		if level, ok := currentLiteral(s.allows[i].ruleFilter, ruleIdx[i]); ok {
			relevant[level] = struct{}{}
			if level == "" {
				includeEmpty = true
			}
		}
		if level, ok := currentLiteral(s.allows[i].tenantFilter, tenantIdx[i]); ok {
			relevant[level] = struct{}{}
			if level == "" {
				includeEmpty = true
			}
		}
	}

	generic := genericLevel(relevant)
	relevant[generic] = struct{}{}
	if includeEmpty {
		relevant[""] = struct{}{}
	} else {
		delete(relevant, "")
	}
	return sortedLevels(relevant)
}

func currentLiteral(pattern []string, idx int) (string, bool) {
	if idx < 0 || idx >= len(pattern) {
		return "", false
	}
	level := pattern[idx]
	if level == "+" || level == "#" {
		return "", false
	}
	return level, true
}

func coverageStateKey(filterIdx int, ruleIdx, tenantIdx []int) string {
	return fmt.Sprintf("%d|%v|%v", filterIdx, ruleIdx, tenantIdx)
}

func matcherCanFinish(pattern []string, idx int) bool {
	if idx < 0 {
		return false
	}
	if idx == len(pattern) {
		return true
	}
	return idx == len(pattern)-1 && pattern[idx] == "#"
}

func matcherIsTerminalHash(pattern []string, idx int) bool {
	return idx == len(pattern)-1 && idx >= 0 && pattern[idx] == "#"
}

func advanceMatcher(pattern []string, idx int, level string) int {
	if idx < 0 || idx >= len(pattern) {
		return -1
	}
	if matcherIsTerminalHash(pattern, idx) {
		return idx
	}
	switch pattern[idx] {
	case "+":
		if level == "" {
			return -1
		}
	case "#":
		return -1
	}
	if pattern[idx] != "+" && pattern[idx] != level {
		return -1
	}
	return idx + 1
}

func genericLevel(existing map[string]struct{}) string {
	base := "__tw_other_level__"
	for i := 0; ; i++ {
		candidate := base
		if i > 0 {
			candidate += strconv.Itoa(i)
		}
		if _, ok := existing[candidate]; !ok {
			return candidate
		}
	}
}

func sortedLevels(levels map[string]struct{}) []string {
	out := make([]string, 0, len(levels))
	for level := range levels {
		out = append(out, level)
	}
	sort.Strings(out)
	return out
}
