// Package policy decides which fields a principal may modify.
package policy

// Principal is the authenticated caller as forwarded by the gateway.
type Principal struct {
	Role     string `json:"role"`
	TenantID string `json:"tenant_id"`
}

// Fields that no role may write, at any depth.
var readOnlyFields = map[string]bool{
	"id":         true,
	"tenant_id":  true,
	"owner_id":   true,
	"role":       true,
	"acl":        true,
	"created_at": true,
	"updated_at": true,
}

// Top-level fields each role may write.
var writableFields = map[string]map[string]bool{
	"viewer": {},
	"editor": {
		"title":    true,
		"summary":  true,
		"body":     true,
		"tags":     true,
		"metadata": true,
	},
	"admin": {
		"title":    true,
		"summary":  true,
		"body":     true,
		"tags":     true,
		"metadata": true,
		"status":   true,
	},
}

// canWrite reports whether role may write the location identified by tokens.
// Permission is governed by the first reference token; the whole-document
// pointer is never writable.
func canWrite(role string, tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	top := tokens[0]
	if readOnlyFields[top] {
		return false
	}
	allowed, ok := writableFields[role]
	if !ok {
		return false
	}
	return allowed[top]
}

// CanWrite reports whether p may write the location identified by tokens.
func CanWrite(p Principal, tokens []string) bool {
	return canWrite(p.Role, tokens)
}
