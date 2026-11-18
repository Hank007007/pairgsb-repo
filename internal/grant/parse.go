// Package grant computes the effective authorization carried by a set of
// credentials presented on a single request. The behaviour is specified in
// docs/grant-merge.md.
package grant

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Kind identifies where a credential came from. Tolerance windows and merge
// rules depend on the source kind.
type Kind string

const (
	KindOIDCAccessToken Kind = "oidc_access_token"
	KindServiceAccount  Kind = "service_account"
	KindDelegated       Kind = "delegated"
	KindAnonymous       Kind = "anonymous"
)

// Valid reports whether k is a known credential kind.
func (k Kind) Valid() bool {
	switch k {
	case KindOIDCAccessToken, KindServiceAccount, KindDelegated, KindAnonymous:
		return true
	default:
		return false
	}
}

// Role is a single role entry carried by a credential. Deny marks the entry as
// a restriction on the role rather than a grant of it.
type Role struct {
	Name string `json:"name"`
	Deny bool   `json:"deny"`
}

// UnmarshalJSON accepts either a bare role name or an object with name and
// deny fields.
func (r *Role) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		if strings.TrimSpace(name) == "" {
			return errors.New("grant: role name is empty")
		}
		r.Name = name
		r.Deny = false
		return nil
	}

	var obj struct {
		Name string `json:"name"`
		Deny bool   `json:"deny"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("grant: invalid role entry: %w", err)
	}
	if strings.TrimSpace(obj.Name) == "" {
		return errors.New("grant: role name is empty")
	}
	r.Name = obj.Name
	r.Deny = obj.Deny
	return nil
}

// Credential is one token or credential presented with a request.
type Credential struct {
	Kind      Kind
	Issuer    string
	Subject   string
	Scopes    []string
	Roles     []Role
	IssuedAt  time.Time
	ExpiresAt time.Time
	// Delegator names the subject that issued this credential to Subject. It is
	// only meaningful for delegated credentials.
	Delegator string
}

// Anonymous returns an anonymous credential. Anonymous credentials cannot be
// produced by parsing; they are constructed explicitly.
func Anonymous(subject string) Credential {
	return Credential{Kind: KindAnonymous, Subject: subject}
}

type rawCredential struct {
	Issuer    string    `json:"issuer"`
	Subject   string    `json:"subject"`
	Scope     string    `json:"scope"`
	Scopes    []string  `json:"scopes"`
	Roles     []Role    `json:"roles"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Delegator string    `json:"delegator"`
}

// Parse decodes a credential payload of the given kind. Scope values keep the
// spelling they had on the wire, in the order they appeared in the scope
// string followed by the scopes array.
func Parse(kind Kind, data []byte) (Credential, error) {
	if !kind.Valid() {
		return Credential{}, fmt.Errorf("grant: unknown credential kind %q", kind)
	}
	if kind == KindAnonymous {
		return Credential{}, errors.New("grant: anonymous credentials are constructed, not parsed")
	}

	var raw rawCredential
	if err := json.Unmarshal(data, &raw); err != nil {
		return Credential{}, fmt.Errorf("grant: decode credential: %w", err)
	}

	scopes := make([]string, 0, len(raw.Scopes)+4)
	for _, field := range strings.Fields(raw.Scope) {
		if field == "" {
			continue
		}
		scopes = append(scopes, field)
	}
	for _, scope := range raw.Scopes {
		if scope == "" {
			continue
		}
		scopes = append(scopes, scope)
	}

	roles := make([]Role, 0, len(raw.Roles))
	for _, role := range raw.Roles {
		if strings.TrimSpace(role.Name) == "" {
			return Credential{}, errors.New("grant: role name is empty")
		}
		roles = append(roles, role)
	}

	return Credential{
		Kind:      kind,
		Issuer:    raw.Issuer,
		Subject:   raw.Subject,
		Scopes:    scopes,
		Roles:     roles,
		IssuedAt:  raw.IssuedAt,
		ExpiresAt: raw.ExpiresAt,
		Delegator: raw.Delegator,
	}, nil
}
