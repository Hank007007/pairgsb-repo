package policy_test

import (
	"strings"
	"testing"

	"snisieve/internal/policy"
)

func TestLoadRulesAndMatchExact(t *testing.T) {
	rules, err := policy.LoadRules(strings.NewReader(`
# a comment

www.example.com
`))
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("got %d rules, want 1", len(rules))
	}
	rule, ok := policy.Match(rules, "www.example.com")
	if !ok {
		t.Fatalf("Match(www.example.com) = false, want true")
	}
	if rule.Name != "www.example.com" {
		t.Errorf("matched rule %q, want www.example.com", rule.Name)
	}
}
