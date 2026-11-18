package grant

import "sort"

// MergeRoles combines the role entries carried by the given credentials into a
// single role list.
func MergeRoles(creds ...Credential) []Role {
	position := make(map[string]int)
	out := make([]Role, 0)

	for _, cred := range creds {
		for _, role := range cred.Roles {
			if role.Name == "" {
				continue
			}
			if i, seen := position[role.Name]; seen {
				out[i] = role
				continue
			}
			position[role.Name] = len(out)
			out = append(out, role)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}
