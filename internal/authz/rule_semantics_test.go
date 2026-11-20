package authz

import "testing"

func TestRuleSubresourceIsWholeSegment(t *testing.T) {
	podRule := Rule{Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"pods"}}
	execRule := Rule{Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"pods/exec"}}

	execReq := Request{Verb: "get", APIGroup: "", Resource: "pods", Subresource: "exec", Namespace: "platform", Name: "web-1"}
	if podRule.Matches(execReq) {
		t.Fatalf("pods rule must not match pods/exec request")
	}
	if !execRule.Matches(execReq) {
		t.Fatalf("pods/exec rule must match pods/exec request")
	}

	podReq := Request{Verb: "get", APIGroup: "", Resource: "pods", Namespace: "platform", Name: "web-1"}
	if !podRule.Matches(podReq) {
		t.Fatalf("pods rule must match plain pods request")
	}
	if execRule.Matches(podReq) {
		t.Fatalf("pods/exec rule must not match plain pods request")
	}
}

func TestRuleResourceWildcardCoversSubresources(t *testing.T) {
	rule := Rule{
		Verbs:     []string{"get"},
		APIGroups: []string{""},
		Resources: []string{"*"},
	}
	cases := []Request{
		{Verb: "get", APIGroup: "", Resource: "pods", Namespace: "data", Name: "p1"},
		{Verb: "get", APIGroup: "", Resource: "pods", Subresource: "exec", Namespace: "data", Name: "p1"},
		{Verb: "get", APIGroup: "", Resource: "pods", Subresource: "log", Namespace: "data", Name: "p1"},
	}
	for _, req := range cases {
		if !rule.Matches(req) {
			t.Fatalf("resources [*] must match %+v", req)
		}
	}
}

func TestRuleResourceWildcardStillConstrainedByVerbAndName(t *testing.T) {
	rule := Rule{
		Verbs:         []string{"get"},
		APIGroups:     []string{""},
		Resources:     []string{"*"},
		ResourceNames: []string{"db-prod"},
	}
	if rule.Matches(Request{Verb: "list", APIGroup: "", Resource: "pods", Namespace: "data"}) {
		t.Fatalf("wildcard resources must not bypass verb and resourceNames constraints")
	}
	if rule.Matches(Request{Verb: "get", APIGroup: "", Resource: "pods", Subresource: "exec", Namespace: "data"}) {
		t.Fatalf("resourceNames rule must not match unnamed subresource request")
	}
	if !rule.Matches(Request{Verb: "get", APIGroup: "", Resource: "pods", Subresource: "exec", Namespace: "data", Name: "db-prod"}) {
		t.Fatalf("wildcard resources must match named subresource request")
	}
}

func TestRuleResourceNamesRejectCollectionLevelRequests(t *testing.T) {
	rule := Rule{
		Verbs:         []string{"get", "list", "watch"},
		APIGroups:     []string{""},
		Resources:     []string{"pods"},
		ResourceNames: []string{"db-prod"},
	}
	for _, verb := range []string{"list", "watch", "create"} {
		req := Request{Verb: verb, APIGroup: "", Resource: "pods", Namespace: "data"}
		if rule.Matches(req) {
			t.Fatalf("resourceNames rule must not match unnamed %s request", verb)
		}
	}
	if !rule.Matches(Request{Verb: "get", APIGroup: "", Resource: "pods", Namespace: "data", Name: "db-prod"}) {
		t.Fatalf("rule must match the named object")
	}
	if rule.Matches(Request{Verb: "get", APIGroup: "", Resource: "pods", Namespace: "data", Name: "db-prod-2"}) {
		t.Fatalf("name comparison must be exact, no prefix matching")
	}
}

func TestRuleResourceNamesWildcardRequiresConcreteName(t *testing.T) {
	rule := Rule{
		Verbs:         []string{"get"},
		APIGroups:     []string{""},
		Resources:     []string{"pods"},
		ResourceNames: []string{"*"},
	}
	if rule.Matches(Request{Verb: "list", APIGroup: "", Resource: "pods", Namespace: "data"}) {
		t.Fatalf("resourceNames [*] must not match collection-level requests")
	}
	if !rule.Matches(Request{Verb: "get", APIGroup: "", Resource: "pods", Namespace: "data", Name: "anything"}) {
		t.Fatalf("resourceNames [*] must match any concrete name")
	}
}
