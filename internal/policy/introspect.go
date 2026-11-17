package policy

import (
	"fmt"

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
			name := s.Name
			if s.Alias != "" {
				name = s.Alias
			}
			if name == "__schema" || name == "__type" {
				return fmt.Errorf("field %q is not available", name)
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
