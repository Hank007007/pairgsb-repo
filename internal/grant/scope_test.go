package grant

import (
	"reflect"
	"testing"
)

func TestUnionSingleCredential(t *testing.T) {
	cred := Credential{
		Kind:   KindOIDCAccessToken,
		Scopes: []string{"openid", "profile"},
	}
	got := Union(cred)
	want := []string{"openid", "profile"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Union = %v, want %v", got, want)
	}
}

func TestUnionTwoCredentialsIsSorted(t *testing.T) {
	a := Credential{Kind: KindOIDCAccessToken, Scopes: []string{"openid", "profile"}}
	b := Credential{Kind: KindOIDCAccessToken, Scopes: []string{"email", "openid"}}

	got := Union(a, b)
	want := []string{"email", "openid", "profile"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Union = %v, want %v", got, want)
	}
}

func TestUnionDeduplicates(t *testing.T) {
	a := Credential{Kind: KindOIDCAccessToken, Scopes: []string{"read", "read"}}
	if got, want := Union(a), []string{"read"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Union = %v, want %v", got, want)
	}
}

func TestFilterSpecialScopesKeepsOfflineAccessOnSingleOIDCToken(t *testing.T) {
	cred := Credential{Kind: KindOIDCAccessToken, Scopes: []string{"openid", "offline_access"}}
	got := FilterSpecialScopes(Union(cred), cred)
	want := []string{"offline_access", "openid"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FilterSpecialScopes = %v, want %v", got, want)
	}
}

func TestFilterSpecialScopesIgnoresAnonymousCounting(t *testing.T) {
	cred := Credential{Kind: KindOIDCAccessToken, Scopes: []string{"openid", "offline_access"}}
	anon := Anonymous("guest")

	got := FilterSpecialScopes(Union(cred, anon), cred, anon)
	want := []string{"offline_access", "openid"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FilterSpecialScopes = %v, want %v", got, want)
	}
}
