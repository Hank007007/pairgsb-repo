package grant

// NarrowDelegated returns the credential narrowed to what its delegator holds.
// The second result reports whether the delegation stays within the authority
// of the delegator.
func NarrowDelegated(del Credential, delegator Credential) (Credential, bool) {
	granted := make(map[string]struct{}, len(delegator.Scopes))
	for _, scope := range delegator.Scopes {
		canonical := lowerScope(scope)
		if canonical == "" {
			continue
		}
		granted[canonical] = struct{}{}
	}

	narrowed := Credential{
		Kind:      del.Kind,
		Issuer:    del.Issuer,
		Subject:   del.Subject,
		Delegator: del.Delegator,
		IssuedAt:  del.IssuedAt,
		ExpiresAt: del.ExpiresAt,
	}

	seenScopes := make(map[string]struct{}, len(del.Scopes))
	for _, scope := range del.Scopes {
		canonical := lowerScope(scope)
		if canonical == "" {
			continue
		}
		if _, ok := granted[canonical]; !ok {
			return Credential{}, false
		}
		if _, seen := seenScopes[canonical]; seen {
			continue
		}
		seenScopes[canonical] = struct{}{}
		narrowed.Scopes = append(narrowed.Scopes, canonical)
	}

	allowedRoles := make(map[string]bool)
	deniedRoles := make(map[string]bool)
	for _, role := range delegator.Roles {
		if role.Name == "" {
			continue
		}
		if role.Deny {
			deniedRoles[role.Name] = true
		} else {
			allowedRoles[role.Name] = true
		}
	}
	for _, role := range del.Roles {
		if role.Deny {
			continue
		}
		if !allowedRoles[role.Name] || deniedRoles[role.Name] {
			return Credential{}, false
		}
	}

	narrowed.Roles = append(narrowed.Roles, del.Roles...)
	return narrowed, true
}
