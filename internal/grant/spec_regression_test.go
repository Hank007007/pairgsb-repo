package grant

import (
	"reflect"
	"testing"
	"time"
)

func TestUnionCanonicalizesScopes(t *testing.T) {
	cred := Credential{
		Kind:   KindOIDCAccessToken,
		Scopes: []string{" Read ", "read", "DOCS:WRITE"},
	}

	got := Union(cred)
	want := []string{"docs:write", "read"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Union = %v, want %v", got, want)
	}
}

func TestFilterSpecialScopesDropsMutuallyExclusiveLosers(t *testing.T) {
	cred := Credential{
		Kind: KindOIDCAccessToken,
		Scopes: []string{
			"docs:read", "docs:write",
			"admin", "readonly",
			"session:ephemeral", "session:persistent",
		},
	}

	got := FilterSpecialScopes(Union(cred), cred)
	want := []string{"admin", "docs:write", "session:persistent"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FilterSpecialScopes = %v, want %v", got, want)
	}
}

func TestMergeRolesDenyWinsRegardlessOfOrder(t *testing.T) {
	allow := Credential{Roles: []Role{{Name: "admin"}}}
	deny := Credential{Roles: []Role{{Name: "admin", Deny: true}}}
	want := []Role{{Name: "admin", Deny: true}}

	if got := MergeRoles(allow, deny); !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeRoles(allow, deny) = %+v, want %+v", got, want)
	}
	if got := MergeRoles(deny, allow); !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeRoles(deny, allow) = %+v, want %+v", got, want)
	}
}

func TestNarrowDelegatedRejectsAllowRoleBeyondDelegator(t *testing.T) {
	delegator := Credential{
		Kind:    KindOIDCAccessToken,
		Subject: "user-1",
		Scopes:  []string{"read"},
		Roles:   []Role{{Name: "reader"}},
	}
	del := Credential{
		Kind:      KindDelegated,
		Subject:   "app-1",
		Delegator: "user-1",
		Scopes:    []string{"read"},
		Roles:     []Role{{Name: "admin"}},
	}

	if _, ok := NarrowDelegated(del, delegator); ok {
		t.Fatal("NarrowDelegated accepted a role the delegator does not hold")
	}
}

func TestNarrowDelegatedKeepsDenyWithoutDelegatorRole(t *testing.T) {
	delegator := Credential{
		Kind:    KindOIDCAccessToken,
		Subject: "user-1",
		Scopes:  []string{"read"},
	}
	del := Credential{
		Kind:      KindDelegated,
		Subject:   "app-1",
		Delegator: "user-1",
		Scopes:    []string{"read"},
		Roles:     []Role{{Name: "admin", Deny: true}},
	}

	narrowed, ok := NarrowDelegated(del, delegator)
	if !ok {
		t.Fatal("NarrowDelegated rejected a deny role")
	}
	want := []Role{{Name: "admin", Deny: true}}
	if !reflect.DeepEqual(narrowed.Roles, want) {
		t.Fatalf("Roles = %+v, want %+v", narrowed.Roles, want)
	}
}

func TestWithinSkewUsesSourceWindowAndInclusiveBoundary(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		kind Kind
		skew time.Duration
	}{
		{KindOIDCAccessToken, 120 * time.Second},
		{KindDelegated, 60 * time.Second},
		{KindServiceAccount, 30 * time.Second},
	}

	for _, tc := range tests {
		t.Run(string(tc.kind), func(t *testing.T) {
			if !WithinSkew(tc.kind, now, now.Add(-tc.skew)) {
				t.Errorf("%s: credential at exp+skew should be valid", tc.kind)
			}
			if WithinSkew(tc.kind, now, now.Add(-tc.skew-time.Second)) {
				t.Errorf("%s: credential past exp+skew should be expired", tc.kind)
			}
		})
	}
}
