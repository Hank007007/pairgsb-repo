package authz

import "testing"

func TestRuleMatchesExactResourceAndName(t *testing.T) {
	rule := Rule{
		Verbs:         []string{"get"},
		APIGroups:     []string{""},
		Resources:     []string{"pods"},
		ResourceNames: []string{"db-prod"},
	}
	req := Request{Verb: "get", APIGroup: "", Resource: "pods", Namespace: "data", Name: "db-prod"}
	if !rule.Matches(req) {
		t.Fatalf("expected rule to match %+v", req)
	}
}

func TestRuleRequiresEveryDimensionToMatch(t *testing.T) {
	rule := Rule{
		Verbs:     []string{"get"},
		APIGroups: []string{"apps"},
		Resources: []string{"deployments"},
	}
	cases := []struct {
		name string
		req  Request
	}{
		{"wrong verb", Request{Verb: "delete", APIGroup: "apps", Resource: "deployments"}},
		{"wrong group", Request{Verb: "get", APIGroup: "", Resource: "deployments"}},
		{"wrong resource", Request{Verb: "get", APIGroup: "apps", Resource: "statefulsets"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if rule.Matches(tc.req) {
				t.Fatalf("expected rule not to match %+v", tc.req)
			}
		})
	}
}

func TestRuleWildcardVerbAndGroup(t *testing.T) {
	rule := Rule{
		Verbs:     []string{"*"},
		APIGroups: []string{"*"},
		Resources: []string{"configmaps"},
	}
	req := Request{Verb: "patch", APIGroup: "apps", Resource: "configmaps", Namespace: "default"}
	if !rule.Matches(req) {
		t.Fatalf("expected wildcard rule to match %+v", req)
	}
}

func TestRuleWithoutResourceNamesAcceptsAnyName(t *testing.T) {
	rule := Rule{
		Verbs:     []string{"get"},
		APIGroups: []string{""},
		Resources: []string{"pods"},
	}
	for _, name := range []string{"", "db-prod", "worker-7"} {
		req := Request{Verb: "get", APIGroup: "", Resource: "pods", Name: name}
		if !rule.Matches(req) {
			t.Fatalf("expected rule to match name %q", name)
		}
	}
}

func TestRuleResourceNamesRejectOtherName(t *testing.T) {
	rule := Rule{
		Verbs:         []string{"get"},
		APIGroups:     []string{""},
		Resources:     []string{"pods"},
		ResourceNames: []string{"db-prod"},
	}
	req := Request{Verb: "get", APIGroup: "", Resource: "pods", Name: "worker-7"}
	if rule.Matches(req) {
		t.Fatalf("expected rule not to match unlisted name")
	}
}

func TestFullResource(t *testing.T) {
	if got := (Request{Resource: "pods"}).FullResource(); got != "pods" {
		t.Fatalf("FullResource() = %q, want %q", got, "pods")
	}
}
