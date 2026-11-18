package grant

import (
	"reflect"
	"testing"
)

func TestNarrowDelegatedKeepsScopesTheDelegatorHolds(t *testing.T) {
	delegator := Credential{
		Kind:    KindOIDCAccessToken,
		Subject: "user-1",
		Scopes:  []string{"read", "write"},
		Roles:   []Role{{Name: "reader"}},
	}
	del := Credential{
		Kind:      KindDelegated,
		Subject:   "app-1",
		Delegator: "user-1",
		Scopes:    []string{"read"},
		Roles:     []Role{{Name: "reader"}},
	}

	narrowed, ok := NarrowDelegated(del, delegator)
	if !ok {
		t.Fatal("NarrowDelegated rejected a delegation within the delegator's authority")
	}
	if !reflect.DeepEqual(narrowed.Scopes, []string{"read"}) {
		t.Errorf("Scopes = %v", narrowed.Scopes)
	}
	if !reflect.DeepEqual(narrowed.Roles, []Role{{Name: "reader"}}) {
		t.Errorf("Roles = %+v", narrowed.Roles)
	}
}

func TestNarrowDelegatedRejectsScopeBeyondDelegator(t *testing.T) {
	delegator := Credential{
		Kind:    KindOIDCAccessToken,
		Subject: "user-1",
		Scopes:  []string{"read"},
	}
	del := Credential{
		Kind:      KindDelegated,
		Subject:   "app-1",
		Delegator: "user-1",
		Scopes:    []string{"read", "billing:admin"},
	}

	if _, ok := NarrowDelegated(del, delegator); ok {
		t.Fatal("NarrowDelegated accepted a scope the delegator does not hold")
	}
}

func TestNarrowDelegatedCarriesDelegatorRolesThrough(t *testing.T) {
	delegator := Credential{
		Kind:    KindServiceAccount,
		Subject: "svc-1",
		Roles:   []Role{{Name: "reader"}, {Name: "writer"}},
	}
	del := Credential{
		Kind:      KindDelegated,
		Subject:   "app-1",
		Delegator: "svc-1",
		Roles:     []Role{{Name: "writer"}},
	}

	narrowed, ok := NarrowDelegated(del, delegator)
	if !ok {
		t.Fatal("NarrowDelegated rejected a delegation within the delegator's authority")
	}
	if !reflect.DeepEqual(narrowed.Roles, []Role{{Name: "writer"}}) {
		t.Errorf("Roles = %+v", narrowed.Roles)
	}
}
