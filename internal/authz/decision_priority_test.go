package authz

import "testing"

func denyRule(verb, group, resource string) Rule {
	return Rule{
		Effect:    EffectDeny,
		Verbs:     []string{verb},
		APIGroups: []string{group},
		Resources: []string{resource},
	}
}

func TestEvaluateDenyOverridesAllow(t *testing.T) {
	req := Request{Verb: "get", APIGroup: "", Resource: "pods", Namespace: "data", Name: "db-prod"}
	identity := Identity{User: "bob", Groups: []string{"system:authenticated"}}

	allowFirst := []Binding{{
		Namespace: "data",
		Subjects:  []Subject{{Kind: "User", Name: "bob"}},
		Rules:     []Rule{allowRule("get", "", "pods"), denyRule("get", "", "pods")},
	}}
	denyFirst := []Binding{{
		Namespace: "data",
		Subjects:  []Subject{{Kind: "User", Name: "bob"}},
		Rules:     []Rule{denyRule("get", "", "pods"), allowRule("get", "", "pods")},
	}}

	for _, bindings := range [][]Binding{allowFirst, denyFirst} {
		result := Evaluate(req, identity, bindings)
		if result.Decision != DecisionDeny {
			t.Fatalf("decision = %s, want deny regardless of rule order", result.Decision)
		}
	}
}

func TestEvaluateDenyAcrossBindingsOverridesAllow(t *testing.T) {
	req := Request{Verb: "get", APIGroup: "", Resource: "pods", Namespace: "data", Name: "db-prod"}
	identity := Identity{User: "bob", Groups: []string{"system:authenticated"}}
	bindings := []Binding{
		{
			Namespace: "data",
			Subjects:  []Subject{{Kind: "Group", Name: "system:authenticated"}},
			Rules:     []Rule{allowRule("get", "", "pods")},
		},
		{
			Namespace: "data",
			Subjects:  []Subject{{Kind: "User", Name: "bob"}},
			Rules:     []Rule{denyRule("get", "", "pods")},
		},
	}
	if result := Evaluate(req, identity, bindings); result.Decision != DecisionDeny {
		t.Fatalf("decision = %s, want deny", result.Decision)
	}
}
