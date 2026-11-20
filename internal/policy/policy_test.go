package policy

import (
	"testing"

	"condgrate/internal/cond"
)

type testEnv map[string]any

func (t testEnv) Get(path string) any { return t[path] }
func (t testEnv) Type(string) (string, bool) {
	return "", false
}

func chainAt(layer Layer, id string) *Chain {
	c := &Chain{}
	scope := Scope{
		Layer: layer,
		Target: Target{
			Principal: MatchBlock{"role": {"analyst"}},
			Action:    []string{"read"},
			Resource:  MatchBlock{"type": {"document"}},
		},
		Policies: []Policy{{
			ID:        id,
			Effect:    Permit,
			Principal: MatchBlock{"role": {"analyst"}},
			Action:    []string{"read"},
			Resource:  MatchBlock{"type": {"document"}},
		}},
	}
	switch layer {
	case OrgLayer:
		c.Org = scope
	case TeamLayer:
		c.Team = scope
	case ProjectLayer:
		c.Project = scope
	}
	return c
}

func TestCollectSingleLayer(t *testing.T) {
	chain := chainAt(ProjectLayer, "project-read")
	req := MatchRequest{
		Principal: MatchBlock{"role": {"analyst"}},
		Resource:  MatchBlock{"type": {"document"}},
		Action:    "read",
	}
	got := Collect(chain, req, testEnv{})
	if len(got) != 1 || got[0].Policy.ID != "project-read" {
		t.Fatalf("got %v, want project-read", got)
	}
	if got[0].Verdict != cond.True {
		t.Fatalf("verdict = %v, want true", got[0].Verdict)
	}
}

func TestCollectTargetMismatch(t *testing.T) {
	chain := chainAt(ProjectLayer, "project-read")
	req := MatchRequest{
		Principal: MatchBlock{"role": {"contractor"}},
		Resource:  MatchBlock{"type": {"document"}},
		Action:    "read",
	}
	if got := Collect(chain, req, testEnv{}); len(got) != 0 {
		t.Fatalf("got %v, want none", got)
	}
}

func TestCombinePermitOnly(t *testing.T) {
	got := Combine([]Candidate{
		{Policy: Policy{ID: "a", Effect: Permit}, Verdict: cond.True},
	})
	if got.Effect != Permit {
		t.Fatalf("effect = %v, want permit", got.Effect)
	}
	if len(got.Obligations) != 0 {
		t.Fatalf("obligations = %v, want none", got.Obligations)
	}
}

func TestCombineNoCandidates(t *testing.T) {
	got := Combine(nil)
	if got.Effect != Deny {
		t.Fatalf("effect = %v, want deny", got.Effect)
	}
}

func TestCombineFalseCandidatesIgnored(t *testing.T) {
	got := Combine([]Candidate{
		{Policy: Policy{ID: "a", Effect: Permit}, Verdict: cond.False},
	})
	if got.Effect != Deny {
		t.Fatalf("effect = %v, want deny", got.Effect)
	}
}
