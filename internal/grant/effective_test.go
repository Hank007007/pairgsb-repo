package grant

import (
	"reflect"
	"testing"
	"time"
)

func TestComputeSingleOIDCToken(t *testing.T) {
	req := Request{
		Now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		Creds: []Credential{{
			Kind:      KindOIDCAccessToken,
			Subject:   "user-1",
			Scopes:    []string{"profile", "openid"},
			Roles:     []Role{{Name: "reader"}},
			ExpiresAt: time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC),
		}},
	}

	got := Compute(req)
	if got.Subject != "user-1" {
		t.Errorf("Subject = %q", got.Subject)
	}
	if !reflect.DeepEqual(got.Scopes, []string{"openid", "profile"}) {
		t.Errorf("Scopes = %v", got.Scopes)
	}
	if !reflect.DeepEqual(got.Roles, []Role{{Name: "reader"}}) {
		t.Errorf("Roles = %+v", got.Roles)
	}
}

func TestComputeExpiredCredentialExcluded(t *testing.T) {
	req := Request{
		Now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		Creds: []Credential{{
			Kind:      KindOIDCAccessToken,
			Subject:   "user-1",
			Scopes:    []string{"openid"},
			ExpiresAt: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
		}},
	}

	got := Compute(req)
	if len(got.Scopes) != 0 {
		t.Errorf("Scopes = %v, want empty", got.Scopes)
	}
}

func TestComputeExpiryTolerantOfRecentSkew(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	req := Request{
		Now: now,
		Creds: []Credential{{
			Kind:      KindOIDCAccessToken,
			Subject:   "user-1",
			Scopes:    []string{"openid"},
			ExpiresAt: now.Add(-time.Minute),
		}},
	}

	got := Compute(req)
	if !reflect.DeepEqual(got.Scopes, []string{"openid"}) {
		t.Errorf("Scopes = %v", got.Scopes)
	}
}

func TestComputeDelegatedCredentialNarrowedByDelegator(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	exp := now.Add(time.Hour)
	req := Request{
		Now: now,
		Creds: []Credential{
			{
				Kind:      KindOIDCAccessToken,
				Subject:   "user-1",
				Scopes:    []string{"openid", "read"},
				Roles:     []Role{{Name: "reader"}},
				ExpiresAt: exp,
			},
			{
				Kind:      KindDelegated,
				Subject:   "app-1",
				Delegator: "user-1",
				Scopes:    []string{"read"},
				ExpiresAt: exp,
			},
		},
	}

	got := Compute(req)
	if !reflect.DeepEqual(got.Scopes, []string{"openid", "read"}) {
		t.Errorf("Scopes = %v", got.Scopes)
	}
}

func TestComputeAnonymousOnly(t *testing.T) {
	got := Compute(Request{Now: time.Now(), Creds: []Credential{Anonymous("guest")}})
	if got.Subject != "" {
		t.Errorf("Subject = %q, want empty", got.Subject)
	}
	if len(got.Scopes) != 0 || len(got.Roles) != 0 {
		t.Errorf("grant = %+v, want empty", got)
	}
}

func TestComputeServiceAccountRoles(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	req := Request{
		Now: now,
		Creds: []Credential{{
			Kind:      KindServiceAccount,
			Subject:   "svc-batch",
			Roles:     []Role{{Name: "writer"}, {Name: "reader"}},
			ExpiresAt: now.Add(time.Hour),
		}},
	}

	got := Compute(req)
	want := []Role{{Name: "reader"}, {Name: "writer"}}
	if !reflect.DeepEqual(got.Roles, want) {
		t.Errorf("Roles = %+v, want %+v", got.Roles, want)
	}
}
