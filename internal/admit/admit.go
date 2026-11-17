// Package admit ties parsing, fragment expansion, policy checks and cost
// accounting together into the decision a gateway makes before a query is
// forwarded to the backend.
package admit

import (
	"fmt"

	"gqladmit/internal/cost"
	"gqladmit/internal/graphql"
	"gqladmit/internal/parse"
	"gqladmit/internal/policy"
)

// Limits are the per-request bounds enforced by the admission layer.
type Limits struct {
	MaxDepth  int
	MaxFields int
}

// Decision describes an admitted request.
type Decision struct {
	Operation string
	Depth     int
	Fields    int
}

// Rejection is returned when a request is denied. Code is a short stable
// identifier suitable for logs and client responses.
type Rejection struct {
	Code    string
	Message string
}

func (r *Rejection) Error() string {
	return fmt.Sprintf("%s: %s", r.Code, r.Message)
}

// Codes reported by Rejection.
const (
	CodeSyntax        = "SYNTAX"
	CodeIntrospection = "INTROSPECTION"
	CodeForbidden     = "FORBIDDEN"
	CodeDepth         = "DEPTH"
	CodeFields        = "FIELDS"
)

// Admitter evaluates requests for a set of tenants.
type Admitter struct {
	Limits   Limits
	Guard    *policy.IntrospectionGuard
	Authoriz *policy.Authorizer
}

// New returns an Admitter with the given limits and policy components.
func New(limits Limits, guard *policy.IntrospectionGuard, authoriz *policy.Authorizer) *Admitter {
	return &Admitter{Limits: limits, Guard: guard, Authoriz: authoriz}
}

// Admit parses a request, resolves its fragments, applies policy and verifies
// its cost. A nil error means the request may be forwarded to the backend.
func (a *Admitter) Admit(tenant, query string) (*Decision, error) {
	doc, err := parse.Parse(query)
	if err != nil {
		return nil, &Rejection{Code: CodeSyntax, Message: err.Error()}
	}

	if len(doc.Operations) != 1 {
		return nil, &Rejection{
			Code:    CodeSyntax,
			Message: fmt.Sprintf("document must contain exactly one operation, found %d", len(doc.Operations)),
		}
	}

	expanded, err := parse.Expand(doc)
	if err != nil {
		return nil, &Rejection{Code: CodeSyntax, Message: err.Error()}
	}
	op := expanded.Operations[0]

	if err := a.Guard.Check(op.SelectionSet); err != nil {
		return nil, &Rejection{Code: CodeIntrospection, Message: err.Error()}
	}

	if err := a.Authoriz.Check(tenant, op.SelectionSet); err != nil {
		return nil, &Rejection{Code: CodeForbidden, Message: err.Error()}
	}

	depth := cost.Depth(op)
	if depth > a.Limits.MaxDepth {
		return nil, &Rejection{
			Code:    CodeDepth,
			Message: fmt.Sprintf("query depth %d exceeds limit %d", depth, a.Limits.MaxDepth),
		}
	}

	fields := cost.Count(op)
	if fields > a.Limits.MaxFields {
		return nil, &Rejection{
			Code:    CodeFields,
			Message: fmt.Sprintf("query selects %d fields, limit is %d", fields, a.Limits.MaxFields),
		}
	}

	return &Decision{
		Operation: operationLabel(op.Operation),
		Depth:     depth,
		Fields:    fields,
	}, nil
}

func operationLabel(op *graphql.Operation) string {
	if op.Name != "" {
		return string(op.Type) + " " + op.Name
	}
	return string(op.Type)
}
