package policy

import (
	"fmt"
	"strings"

	"gqladmit/internal/graphql"
)

// IntrospectionGuard blocks meta fields that expose a server's schema.
//
// Only __typename is meaningful to ordinary callers: it reports the concrete
// type of the enclosing object and says nothing about the schema itself.
// Every other meta field is an administrative surface and must not be
// reachable through the public gateway.
type IntrospectionGuard struct{}

// NewIntrospectionGuard returns a guard for the public gateway.
func NewIntrospectionGuard() *IntrospectionGuard {
	return &IntrospectionGuard{}
}

// Check walks the given selection tree and reports the first meta field that
// the caller may not use.
func (g *IntrospectionGuard) Check(sels []graphql.Selection) error {
	for _, sel := range sels {
		switch s := sel.(type) {
		case *graphql.Field:
			// The decision is made on the real field name; an alias is
			// only a response key and must not hide a meta field.
			if strings.HasPrefix(s.Name, "__") && s.Name != "__typename" {
				return fmt.Errorf("field %q is not available", s.Name)
			}
			if err := g.Check(s.SelectionSet); err != nil {
				return err
			}
		case *graphql.InlineFragment:
			if err := g.Check(s.SelectionSet); err != nil {
				return err
			}
		}
	}
	return nil
}
