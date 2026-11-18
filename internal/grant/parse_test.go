package grant

import (
	"testing"
	"time"
)

func TestParseCollectsScopeStringAndArray(t *testing.T) {
	data := []byte(`{
		"issuer": "https://idp.example",
		"subject": "user-1",
		"scope": "openid profile",
		"scopes": ["email"],
		"expires_at": "2030-01-01T00:00:00Z"
	}`)

	cred, err := Parse(KindOIDCAccessToken, data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cred.Kind != KindOIDCAccessToken {
		t.Errorf("Kind = %q", cred.Kind)
	}
	if cred.Subject != "user-1" {
		t.Errorf("Subject = %q", cred.Subject)
	}
	want := []string{"openid", "profile", "email"}
	if len(cred.Scopes) != len(want) {
		t.Fatalf("Scopes = %v, want %v", cred.Scopes, want)
	}
	for i := range want {
		if cred.Scopes[i] != want[i] {
			t.Fatalf("Scopes = %v, want %v", cred.Scopes, want)
		}
	}
	if !cred.ExpiresAt.Equal(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("ExpiresAt = %v", cred.ExpiresAt)
	}
}

func TestParseRoles(t *testing.T) {
	data := []byte(`{
		"subject": "svc-batch",
		"roles": ["reader", {"name": "writer", "deny": true}]
	}`)

	cred, err := Parse(KindServiceAccount, data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cred.Roles) != 2 {
		t.Fatalf("Roles = %v", cred.Roles)
	}
	if cred.Roles[0] != (Role{Name: "reader"}) {
		t.Errorf("Roles[0] = %+v", cred.Roles[0])
	}
	if cred.Roles[1] != (Role{Name: "writer", Deny: true}) {
		t.Errorf("Roles[1] = %+v", cred.Roles[1])
	}
}

func TestParseRejectsMalformedInput(t *testing.T) {
	cases := []struct {
		name string
		kind Kind
		data string
	}{
		{"unknown kind", Kind("saml_assertion"), `{}`},
		{"anonymous payload", KindAnonymous, `{}`},
		{"empty role name", KindServiceAccount, `{"roles": ["ok", {"name": "  "}]}`},
		{"bad json", KindOIDCAccessToken, `{"scope":`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse(tc.kind, []byte(tc.data)); err == nil {
				t.Fatal("Parse accepted malformed input")
			}
		})
	}
}

func TestParseDropsEmptyScopes(t *testing.T) {
	cred, err := Parse(KindOIDCAccessToken, []byte(`{"scope": "openid  ", "scopes": ["", "email"]}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cred.Scopes) != 2 {
		t.Fatalf("Scopes = %v", cred.Scopes)
	}
}
