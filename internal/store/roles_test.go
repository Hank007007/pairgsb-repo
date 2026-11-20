package store

import (
	"testing"

	"verbgate/internal/authz"
)

func testSnapshot() Snapshot {
	return Snapshot{
		Version: "v1",
		Roles: []Role{
			{
				Name: "pod-reader",
				Rules: []authz.Rule{
					{Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"pods"}},
				},
			},
		},
		ClusterRoles: []Role{
			{Name: "baseline", Rules: []authz.Rule{
				{Verbs: []string{"list"}, APIGroups: []string{""}, Resources: []string{"namespaces"}},
			}},
		},
		RoleBindings: []Binding{
			{
				Name:      "readers",
				Namespace: "data",
				RoleRef:   RoleRef{Kind: "Role", Name: "pod-reader"},
				Subjects:  []Subject{{Kind: "User", Name: "alice"}},
			},
			{
				Name:      "elsewhere",
				Namespace: "staging",
				RoleRef:   RoleRef{Kind: "Role", Name: "pod-reader"},
				Subjects:  []Subject{{Kind: "User", Name: "alice"}},
			},
		},
		ClusterBindings: []Binding{
			{
				Name:     "baseline-all",
				RoleRef:  RoleRef{Kind: "ClusterRole", Name: "baseline"},
				Subjects: []Subject{{Kind: "Group", Name: "system:authenticated"}},
			},
		},
	}
}

func TestBindingsForNamespaceScopesRoleBindings(t *testing.T) {
	s := New()
	s.Replace(testSnapshot())
	bindings := s.BindingsForNamespace("data")
	if len(bindings) != 2 {
		t.Fatalf("got %d bindings, want 2 (one namespaced, one cluster)", len(bindings))
	}
	for _, binding := range bindings {
		if binding.Namespace == "staging" {
			t.Fatalf("binding from staging leaked into data")
		}
	}
}

func TestBindingsForClusterScopeUsesClusterBindingsOnly(t *testing.T) {
	s := New()
	s.Replace(testSnapshot())
	bindings := s.BindingsForNamespace("")
	if len(bindings) != 1 || bindings[0].Name != "baseline-all" {
		t.Fatalf("unexpected cluster-scoped bindings: %+v", bindings)
	}
}

func TestResolveRole(t *testing.T) {
	s := New()
	s.Replace(testSnapshot())
	role, ok := s.ResolveRole(Binding{RoleRef: RoleRef{Kind: "Role", Name: "pod-reader"}})
	if !ok || role.Name != "pod-reader" {
		t.Fatalf("expected pod-reader role, got %+v ok=%v", role, ok)
	}
	if _, ok := s.ResolveRole(Binding{RoleRef: RoleRef{Kind: "Role", Name: "missing"}}); ok {
		t.Fatalf("expected missing role to be unresolved")
	}
}

func TestReplaceSwapsVersion(t *testing.T) {
	s := New()
	s.Replace(testSnapshot())
	next := testSnapshot()
	next.Version = "v2"
	s.Replace(next)
	if got := s.Version(); got != "v2" {
		t.Fatalf("version = %q, want v2", got)
	}
}
