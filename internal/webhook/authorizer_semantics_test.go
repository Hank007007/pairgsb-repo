package webhook

import (
	"testing"

	"verbgate/internal/authz"
	"verbgate/internal/cache"
	"verbgate/internal/store"
	"verbgate/internal/subject"
)

func semanticsAuthorizer(t *testing.T, snapshot store.Snapshot) *Authorizer {
	t.Helper()
	dir, err := subject.ParseDirectory(`
users:
  alice:
    - platform
  bob:
    - db-oncall
groups:
  platform:
    - sre
  sre:
    - db-oncall
  db-oncall: []
`)
	if err != nil {
		t.Fatalf("parse groups: %v", err)
	}
	snapshotStore := store.New()
	snapshotStore.Replace(snapshot)
	return New(snapshotStore, dir, cache.New(16))
}

func semanticsSnapshot() store.Snapshot {
	return store.Snapshot{
		Version: "v1",
		Roles: []store.Role{
			{
				Name: "pod-reader",
				Rules: []authz.Rule{
					{Verbs: []string{"get", "list", "watch"}, APIGroups: []string{""}, Resources: []string{"pods"}},
				},
			},
			{
				Name: "db-operator",
				Rules: []authz.Rule{
					{
						Verbs:         []string{"get", "list", "watch"},
						APIGroups:     []string{""},
						Resources:     []string{"pods"},
						ResourceNames: []string{"db-prod"},
					},
				},
			},
			{
				Name: "exec-er",
				Rules: []authz.Rule{
					{Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"pods/exec"}},
				},
			},
			{
				Name: "wildcard",
				Rules: []authz.Rule{
					{Verbs: []string{"get"}, APIGroups: []string{"*"}, Resources: []string{"*"}},
				},
			},
			{
				Name: "deny-pods",
				Rules: []authz.Rule{
					{Effect: authz.EffectDeny, Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"pods"}},
				},
			},
		},
		ClusterRoles: []store.Role{
			{
				Name: "baseline",
				Rules: []authz.Rule{
					{Verbs: []string{"get", "list"}, APIGroups: []string{""}, Resources: []string{"namespaces"}},
				},
			},
		},
		RoleBindings: []store.Binding{
			{
				Name:      "sre-pod-readers",
				Namespace: "platform",
				RoleRef:   store.RoleRef{Kind: "Role", Name: "pod-reader"},
				Subjects:  []store.Subject{{Kind: "Group", Name: "sre"}},
			},
			{
				Name:      "db-operator-binding",
				Namespace: "data",
				RoleRef:   store.RoleRef{Kind: "Role", Name: "db-operator"},
				Subjects:  []store.Subject{{Kind: "User", Name: "bob"}},
			},
			{
				Name:      "nested-group-exec",
				Namespace: "data",
				RoleRef:   store.RoleRef{Kind: "Role", Name: "exec-er"},
				Subjects:  []store.Subject{{Kind: "Group", Name: "db-oncall"}},
			},
			{
				Name:      "bob-deny",
				Namespace: "data",
				RoleRef:   store.RoleRef{Kind: "Role", Name: "deny-pods"},
				Subjects:  []store.Subject{{Kind: "User", Name: "bob"}},
			},
		},
		ClusterBindings: []store.Binding{
			{
				Name:     "baseline-for-authenticated",
				RoleRef:  store.RoleRef{Kind: "ClusterRole", Name: "baseline"},
				Subjects: []store.Subject{{Kind: "Group", Name: "system:authenticated"}},
			},
		},
	}
}

func TestSubresourceNotImpliedByResourceRule(t *testing.T) {
	a := semanticsAuthorizer(t, semanticsSnapshot())
	resp := a.Authorize(authz.SubjectAccessReview{
		User: "alice", Verb: "get", APIGroup: "", Resource: "pods",
		Subresource: "exec", Namespace: "platform", Name: "web-1",
	})
	if resp.Allowed || resp.Denied {
		t.Fatalf("pods read must not grant pods/exec, got %+v", resp)
	}
}

func TestResourceWildcardCoversSubresources(t *testing.T) {
	snapshot := semanticsSnapshot()
	snapshot.RoleBindings = append(snapshot.RoleBindings, store.Binding{
		Name:      "wildcard-binding",
		Namespace: "platform",
		RoleRef:   store.RoleRef{Kind: "Role", Name: "wildcard"},
		Subjects:  []store.Subject{{Kind: "User", Name: "alice"}},
	})
	a := semanticsAuthorizer(t, snapshot)
	resp := a.Authorize(authz.SubjectAccessReview{
		User: "alice", Verb: "get", APIGroup: "", Resource: "pods",
		Subresource: "log", Namespace: "platform", Name: "web-1",
	})
	if !resp.Allowed {
		t.Fatalf("resources [*] must cover pods/log, got %+v", resp)
	}
}

func TestResourceNamesDoNotGrantCollectionLevelAccess(t *testing.T) {
	snapshot := semanticsSnapshot()
	filtered := make([]store.Binding, 0, len(snapshot.RoleBindings))
	for _, binding := range snapshot.RoleBindings {
		if binding.Name != "bob-deny" {
			filtered = append(filtered, binding)
		}
	}
	snapshot.RoleBindings = filtered
	a := semanticsAuthorizer(t, snapshot)
	named := a.Authorize(authz.SubjectAccessReview{
		User: "bob", Verb: "get", APIGroup: "", Resource: "pods",
		Namespace: "data", Name: "db-prod",
	})
	if !named.Allowed {
		t.Fatalf("bob should read db-prod, got %+v", named)
	}
	listed := a.Authorize(authz.SubjectAccessReview{
		User: "bob", Verb: "list", APIGroup: "", Resource: "pods", Namespace: "data",
	})
	if listed.Allowed || listed.Denied {
		t.Fatalf("resourceNames rule must not grant unnamed list, got %+v", listed)
	}
}

func TestAuthenticatedBuiltinGroupReceivesBaseline(t *testing.T) {
	a := semanticsAuthorizer(t, semanticsSnapshot())
	resp := a.Authorize(authz.SubjectAccessReview{
		User: "carol", Verb: "get", APIGroup: "", Resource: "namespaces", Name: "default",
	})
	if !resp.Allowed {
		t.Fatalf("system:authenticated baseline must apply, got %+v", resp)
	}
}

func TestNestedGroupExpansionGrantsTransitiveRole(t *testing.T) {
	a := semanticsAuthorizer(t, semanticsSnapshot())
	resp := a.Authorize(authz.SubjectAccessReview{
		User: "alice", Verb: "get", APIGroup: "", Resource: "pods",
		Subresource: "exec", Namespace: "data", Name: "web-1",
	})
	if !resp.Allowed {
		t.Fatalf("alice reaches db-oncall via platform -> sre, got %+v", resp)
	}
}

func TestDenyWinsOverAllow(t *testing.T) {
	a := semanticsAuthorizer(t, semanticsSnapshot())
	resp := a.Authorize(authz.SubjectAccessReview{
		User: "bob", Verb: "get", APIGroup: "", Resource: "pods",
		Namespace: "data", Name: "db-prod",
	})
	if !resp.Denied {
		t.Fatalf("deny rule must win over the allow rule, got %+v", resp)
	}
}

func TestCacheDoesNotLeakAcrossUsers(t *testing.T) {
	a := semanticsAuthorizer(t, semanticsSnapshot())
	alice := a.Authorize(authz.SubjectAccessReview{
		User: "alice", Verb: "get", APIGroup: "", Resource: "pods",
		Namespace: "platform", Name: "p1",
	})
	if !alice.Allowed {
		t.Fatalf("alice setup expected allow, got %+v", alice)
	}
	bob := a.Authorize(authz.SubjectAccessReview{
		User: "bob", Verb: "get", APIGroup: "", Resource: "pods",
		Namespace: "platform", Name: "p1",
	})
	if bob.Allowed || bob.Denied {
		t.Fatalf("bob must not inherit alice's cached allow, got %+v", bob)
	}
}

func TestCacheSeparatesImpersonation(t *testing.T) {
	a := semanticsAuthorizer(t, semanticsSnapshot())
	alice := a.Authorize(authz.SubjectAccessReview{
		User: "alice", Verb: "get", APIGroup: "", Resource: "pods",
		Namespace: "platform", Name: "p1",
	})
	if !alice.Allowed {
		t.Fatalf("alice setup expected allow, got %+v", alice)
	}
	impersonated := a.Authorize(authz.SubjectAccessReview{
		User: "alice", Verb: "get", APIGroup: "", Resource: "pods",
		Namespace: "platform", Name: "p1",
		Impersonation: &authz.Impersonation{User: "bob"},
	})
	if impersonated.Allowed || impersonated.Denied {
		t.Fatalf("impersonated request must not reuse alice's entry, got %+v", impersonated)
	}
}

func TestSnapshotSwapInvalidatesCachedDecisions(t *testing.T) {
	dir, err := subject.ParseDirectory("users:\n  alice: []\ngroups:\n")
	if err != nil {
		t.Fatalf("parse groups: %v", err)
	}
	snapshotStore := store.New()
	snapshotStore.Replace(store.Snapshot{
		Version: "v1",
		Roles: []store.Role{{
			Name:  "pod-reader",
			Rules: []authz.Rule{{Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"pods"}}},
		}},
		RoleBindings: []store.Binding{{
			Name: "readers", Namespace: "data",
			RoleRef:  store.RoleRef{Kind: "Role", Name: "pod-reader"},
			Subjects: []store.Subject{{Kind: "User", Name: "alice"}},
		}},
	})
	a := New(snapshotStore, dir, cache.New(16))
	review := authz.SubjectAccessReview{
		User: "alice", Verb: "get", APIGroup: "", Resource: "pods",
		Namespace: "data", Name: "p1",
	}
	if resp := a.Authorize(review); !resp.Allowed {
		t.Fatalf("v1 expected allow, got %+v", resp)
	}

	snapshotStore.Replace(store.Snapshot{Version: "v2"})
	resp := a.Authorize(review)
	if resp.Allowed || resp.Denied {
		t.Fatalf("after snapshot swap the stale allow must not be returned, got %+v", resp)
	}
}
