package grant

import "sort"

// MergeRoles combines the role entries carried by the given credentials into a
// single role list.
func MergeRoles(creds ...Credential) []Role {
	denied := make(map[string]bool)
	names := make(map[string]struct{})

	for _, cred := range creds {
		for _, role := range cred.Roles {
			if role.Name == "" {
				continue
			}
			names[role.Name] = struct{}{}
			denied[role.Name] = denied[role.Name] || role.Deny
		}
	}

	out := make([]Role, 0, len(names))
	for name := range names {
		out = append(out, Role{Name: name, Deny: denied[name]})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}
