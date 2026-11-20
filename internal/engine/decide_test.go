package engine

import (
	"testing"

	"condgrate/internal/attr"
	"condgrate/internal/policy"
)

func testChain(t *testing.T) *policy.Chain {
	t.Helper()
	raw := []byte(`{
	  "org": {
	    "name": "acme",
	    "target": {"principal": {"role": ["analyst"]}, "action": ["read"], "resource": {"type": ["document"]}},
	    "policies": []
	  },
	  "team": {
	    "name": "research",
	    "target": {"principal": {"role": ["analyst"]}, "action": ["read"], "resource": {"type": ["document"]}},
	    "policies": []
	  },
	  "project": {
	    "name": "atlas",
	    "target": {"principal": {"role": ["analyst"]}, "action": ["read"], "resource": {"type": ["document"]}},
	    "policies": [
	      {
	        "id": "project-read",
	        "effect": "permit",
	        "principal": {"role": ["analyst"]},
	        "action": ["read"],
	        "resource": {"type": ["document"]}
	      }
	    ]
	  }
	}`)
	chain, err := policy.ParseChain(raw)
	if err != nil {
		t.Fatalf("parse chain: %v", err)
	}
	return chain
}

func TestDecidePermit(t *testing.T) {
	dir := attr.NewDirectory()
	dir.SetPrincipal(map[string]any{"role": "analyst"}, map[string]string{"role": "string"})
	dir.SetResource(map[string]any{"type": "document"}, map[string]string{"type": "string"})

	eng := New(testChain(t), dir)
	resp := eng.Decide(Request{
		Action:    "read",
		Principal: map[string]any{"role": "analyst"},
		Resource:  map[string]any{"type": "document"},
	})
	if resp.Effect != policy.Permit {
		t.Fatalf("effect = %v, reason = %s", resp.Effect, resp.Reason)
	}
	if len(resp.Matched) != 1 || resp.Matched[0] != "project-read" {
		t.Fatalf("matched = %v", resp.Matched)
	}
}

func TestDecideDefaultDeny(t *testing.T) {
	eng := New(testChain(t), attr.NewDirectory())
	resp := eng.Decide(Request{
		Action:    "delete",
		Principal: map[string]any{"role": "analyst"},
		Resource:  map[string]any{"type": "document"},
	})
	if resp.Effect != policy.Deny {
		t.Fatalf("effect = %v, want deny", resp.Effect)
	}
}

func TestDecideObligationsOnPermit(t *testing.T) {
	raw := []byte(`{
	  "org": {"name": "acme", "target": {"principal": {"role": ["analyst"]}, "action": ["read"], "resource": {"type": ["document"]}}, "policies": []},
	  "team": {"name": "research", "target": {"principal": {"role": ["analyst"]}, "action": ["read"], "resource": {"type": ["document"]}}, "policies": []},
	  "project": {"name": "atlas", "target": {"principal": {"role": ["analyst"]}, "action": ["read"], "resource": {"type": ["document"]}}, "policies": [
	    {"id": "audited-read", "effect": "permit", "principal": {"role": ["analyst"]}, "action": ["read"], "resource": {"type": ["document"]}, "obligations": ["audit-log"]}
	  ]}
	}`)
	chain, err := policy.ParseChain(raw)
	if err != nil {
		t.Fatalf("parse chain: %v", err)
	}
	dir := attr.NewDirectory()
	dir.SetPrincipal(map[string]any{"role": "analyst"}, nil)
	dir.SetResource(map[string]any{"type": "document"}, nil)

	resp := New(chain, dir).Decide(Request{
		Action:    "read",
		Principal: map[string]any{"role": "analyst"},
		Resource:  map[string]any{"type": "document"},
	})
	if resp.Effect != policy.Permit {
		t.Fatalf("effect = %v", resp.Effect)
	}
	if len(resp.Obligations) != 1 || resp.Obligations[0] != "audit-log" {
		t.Fatalf("obligations = %v", resp.Obligations)
	}
}
