package engine

import (
	"testing"

	"condgrate/internal/attr"
	"condgrate/internal/policy"
)

func parseChain(t *testing.T, raw string) *policy.Chain {
	t.Helper()
	chain, err := policy.ParseChain([]byte(raw))
	if err != nil {
		t.Fatalf("parse chain: %v", err)
	}
	return chain
}

func TestDecideIgnoresRequestClearanceOverride(t *testing.T) {
	chain := parseChain(t, `{
	  "org": {"name": "acme", "target": {"principal": {"role": ["*"]}, "action": ["read"], "resource": {"type": ["*"]}}, "policies": []},
	  "team": {"name": "research", "target": {"principal": {"role": ["*"]}, "action": ["read"], "resource": {"type": ["*"]}}, "policies": []},
	  "project": {"name": "atlas", "target": {"principal": {"role": ["*"]}, "action": ["read"], "resource": {"type": ["document"]}}, "policies": [
	    {"id": "restricted-read", "effect": "permit", "principal": {"role": ["*"]}, "action": ["read"], "resource": {"classification": ["restricted"]},
	     "condition": {"cmp": {"op": "gte", "left": {"attr": "principal.clearance"}, "right": 5}}}
	  ]}
	}`)

	dir := attr.NewDirectory()
	dir.SetPrincipal(
		map[string]any{"role": "analyst", "clearance": float64(2)},
		map[string]string{"role": "string", "clearance": "number"},
	)
	dir.SetResource(
		map[string]any{"type": "document", "classification": "restricted"},
		map[string]string{"type": "string", "classification": "string"},
	)

	resp := New(chain, dir).Decide(Request{
		Action:    "read",
		Principal: map[string]any{"role": "analyst", "clearance": "9"},
		Resource:  map[string]any{"type": "document", "classification": "restricted"},
	})
	if resp.Effect != policy.Deny {
		t.Fatalf("effect = %v, want deny using directory clearance", resp.Effect)
	}
}

func TestDecideOrgDenyOverridesProjectPermit(t *testing.T) {
	chain := parseChain(t, `{
	  "org": {"name": "acme", "target": {"principal": {"role": ["*"]}, "action": ["read"], "resource": {"type": ["*"]}}, "policies": [
	    {"id": "org-deny-contractors", "effect": "deny", "principal": {"role": ["contractor"]}, "action": ["read"], "resource": {"type": ["*"]}}
	  ]},
	  "team": {"name": "research", "target": {"principal": {"role": ["*"]}, "action": ["read"], "resource": {"type": ["*"]}}, "policies": []},
	  "project": {"name": "atlas", "target": {"principal": {"role": ["*"]}, "action": ["read"], "resource": {"type": ["*"]}}, "policies": [
	    {"id": "project-read", "effect": "permit", "principal": {"role": ["contractor"]}, "action": ["read"], "resource": {"type": ["document"]}, "obligations": ["audit-log"]}
	  ]}
	}`)

	dir := attr.NewDirectory()
	dir.SetPrincipal(map[string]any{"role": "contractor"}, map[string]string{"role": "string"})
	dir.SetResource(map[string]any{"type": "document"}, map[string]string{"type": "string"})

	resp := New(chain, dir).Decide(Request{
		Action:    "read",
		Principal: map[string]any{"role": "contractor"},
		Resource:  map[string]any{"type": "document"},
	})
	if resp.Effect != policy.Deny {
		t.Fatalf("effect = %v, want deny", resp.Effect)
	}
	if len(resp.Obligations) != 0 {
		t.Fatalf("obligations = %v, want none", resp.Obligations)
	}
}

func TestDecideIndeterminateFailsClosed(t *testing.T) {
	chain := parseChain(t, `{
	  "org": {"name": "acme", "target": {"principal": {"role": ["*"]}, "action": ["read"], "resource": {"type": ["*"]}}, "policies": []},
	  "team": {"name": "research", "target": {"principal": {"role": ["*"]}, "action": ["read"], "resource": {"type": ["*"]}}, "policies": []},
	  "project": {"name": "atlas", "target": {"principal": {"role": ["*"]}, "action": ["read"], "resource": {"type": ["*"]}}, "policies": [
	    {"id": "audited-read", "effect": "permit", "principal": {"role": ["*"]}, "action": ["read"], "resource": {"type": ["document"]}, "obligations": ["audit-log"]},
	    {"id": "unknown-tags", "effect": "permit", "principal": {"role": ["*"]}, "action": ["read"], "resource": {"type": ["document"]},
	     "condition": {"not": {"contains": {"path": "resource.tags", "value": "pii"}}}}
	  ]}
	}`)

	dir := attr.NewDirectory()
	dir.SetPrincipal(map[string]any{"role": "analyst"}, map[string]string{"role": "string"})
	dir.SetResource(map[string]any{"type": "document"}, map[string]string{"type": "string"})

	resp := New(chain, dir).Decide(Request{
		Action:    "read",
		Principal: map[string]any{"role": "analyst"},
		Resource:  map[string]any{"type": "document"},
	})
	if resp.Effect != policy.Deny {
		t.Fatalf("effect = %v, want deny for indeterminate", resp.Effect)
	}
	if !resp.Indeterminate {
		t.Fatal("Indeterminate = false, want true")
	}
	if len(resp.Obligations) != 0 {
		t.Fatalf("obligations = %v, want none", resp.Obligations)
	}
}
