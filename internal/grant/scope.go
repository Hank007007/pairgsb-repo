package grant

import (
	"sort"
	"strings"
)

// OfflineAccess is the authorization-management scope that is only carried
// through under the conditions described in the design document.
const OfflineAccess = "offline_access"

// Union returns the scope union of the given credentials.
func Union(creds ...Credential) []string {
	set := make(map[string]struct{})
	for _, cred := range creds {
		for _, scope := range cred.Scopes {
			canonical := lowerScope(scope)
			if canonical == "" {
				continue
			}
			set[canonical] = struct{}{}
		}
	}

	out := make([]string, 0, len(set))
	for scope := range set {
		out = append(out, scope)
	}
	sort.Strings(out)
	return out
}

// FilterSpecialScopes applies the special-scope rules to a scope set that has
// already been merged from creds.
func FilterSpecialScopes(scopes []string, creds ...Credential) []string {
	contributors := 0
	onlyKind := Kind("")
	for _, cred := range creds {
		if cred.Kind == KindAnonymous {
			continue
		}
		hasScope := false
		for _, scope := range cred.Scopes {
			if lowerScope(scope) != "" {
				hasScope = true
				break
			}
		}
		if !hasScope {
			continue
		}
		contributors++
		onlyKind = cred.Kind
	}

	keepOffline := contributors == 1 && onlyKind == KindOIDCAccessToken

	set := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		canonical := lowerScope(scope)
		if canonical == "" {
			continue
		}
		set[canonical] = struct{}{}
	}
	if !keepOffline {
		delete(set, OfflineAccess)
	}

	conflicts := [][2]string{
		{"docs:write", "docs:read"},
		{"admin", "readonly"},
		{"session:persistent", "session:ephemeral"},
	}
	for _, pair := range conflicts {
		keeper, discarded := pair[0], pair[1]
		if _, keeperPresent := set[keeper]; keeperPresent {
			delete(set, discarded)
		}
	}

	out := make([]string, 0, len(set))
	for scope := range set {
		out = append(out, scope)
	}
	sort.Strings(out)
	return out
}

// lowerScope is the comparison form of a scope value.
func lowerScope(scope string) string {
	return strings.ToLower(strings.TrimSpace(scope))
}
