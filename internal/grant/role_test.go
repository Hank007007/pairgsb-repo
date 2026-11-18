package grant

import (
	"reflect"
	"testing"
)

func TestMergeRolesCombinesDistinctNames(t *testing.T) {
	a := Credential{Roles: []Role{{Name: "reader"}}}
	b := Credential{Roles: []Role{{Name: "auditor"}}}

	got := MergeRoles(a, b)
	want := []Role{{Name: "auditor"}, {Name: "reader"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeRoles = %+v, want %+v", got, want)
	}
}

func TestMergeRolesDeduplicatesIdenticalEntries(t *testing.T) {
	a := Credential{Roles: []Role{{Name: "reader"}}}
	b := Credential{Roles: []Role{{Name: "reader"}}}

	got := MergeRoles(a, b)
	want := []Role{{Name: "reader"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeRoles = %+v, want %+v", got, want)
	}
}

func TestMergeRolesKeepsNamesCaseSensitive(t *testing.T) {
	a := Credential{Roles: []Role{{Name: "Reader"}}}
	b := Credential{Roles: []Role{{Name: "reader"}}}

	got := MergeRoles(a, b)
	want := []Role{{Name: "Reader"}, {Name: "reader"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeRoles = %+v, want %+v", got, want)
	}
}
