package policy

import (
	"fmt"

	"gqladmit/internal/graphql"
)

// Authorizer decides which schema fields a tenant may read. Access is granted
// per field name: a field listed as restricted is readable only by tenants
// that were explicitly granted it.
type Authorizer struct {
	restricted map[string]bool
	grants     map[string]map[string]bool
}

// NewAuthorizer builds an authorizer from the set of restricted field names
// and a per-tenant list of granted fields.
func NewAuthorizer(restricted []string, grants map[string][]string) *Authorizer {
	a := &Authorizer{
		restricted: make(map[string]bool, len(restricted)),
		grants:     make(map[string]map[string]bool, len(grants)),
	}
	for _, name := range restricted {
		a.restricted[name] = true
	}
	for tenant, fields := range grants {
		set := make(map[string]bool, len(fields))
		for _, f := range fields {
			set[f] = true
		}
		a.grants[tenant] = set
	}
	return a
}

// Check verifies that the tenant may select every field in the tree. It
// returns an error naming the first field the tenant is not entitled to.
func (a *Authorizer) Check(tenant string, sels []graphql.Selection) error {
	for _, sel := range sels {
		switch s := sel.(type) {
		case *graphql.Field:
			// The decision is made on the real field name; an alias does
			// not rename the schema field for authorisation purposes.
			if !a.allowed(tenant, s.Name) {
				return fmt.Errorf("tenant %q may not read field %q", tenant, s.Name)
			}
			if err := a.Check(tenant, s.SelectionSet); err != nil {
				return err
			}
		case *graphql.InlineFragment:
			if err := a.Check(tenant, s.SelectionSet); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *Authorizer) allowed(tenant, field string) bool {
	if !a.restricted[field] {
		return true
	}
	return a.grants[tenant][field]
}
