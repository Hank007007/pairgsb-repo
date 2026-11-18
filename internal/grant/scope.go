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
			if scope == "" {
				continue
			}
			set[scope] = struct{}{}
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
		if cred.Kind == KindAnonymous || len(cred.Scopes) == 0 {
			continue
		}
		contributors++
		onlyKind = cred.Kind
	}

	keepOffline := contributors == 1 && onlyKind == KindOIDCAccessToken

	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if lowerScope(scope) == OfflineAccess && !keepOffline {
			continue
		}
		out = append(out, scope)
	}
	sort.Strings(out)
	return out
}

// lowerScope is the comparison form of a scope value.
func lowerScope(scope string) string {
	return strings.ToLower(strings.TrimSpace(scope))
}
