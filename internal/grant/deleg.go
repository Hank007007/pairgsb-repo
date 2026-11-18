package grant

// NarrowDelegated returns the credential narrowed to what its delegator holds.
// The second result reports whether the delegation stays within the authority
// of the delegator.
func NarrowDelegated(del Credential, delegator Credential) (Credential, bool) {
	granted := make(map[string]struct{}, len(delegator.Scopes))
	for _, scope := range delegator.Scopes {
		granted[lowerScope(scope)] = struct{}{}
	}

	narrowed := Credential{
		Kind:      del.Kind,
		Issuer:    del.Issuer,
		Subject:   del.Subject,
		Delegator: del.Delegator,
		IssuedAt:  del.IssuedAt,
		ExpiresAt: del.ExpiresAt,
	}

	for _, scope := range del.Scopes {
		if _, ok := granted[lowerScope(scope)]; !ok {
			return Credential{}, false
		}
		narrowed.Scopes = append(narrowed.Scopes, scope)
	}

	narrowed.Roles = append(narrowed.Roles, del.Roles...)
	return narrowed, true
}
