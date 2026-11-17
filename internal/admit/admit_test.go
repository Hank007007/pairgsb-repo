package admit_test

import (
	"errors"
	"strings"
	"testing"

	"gqladmit/internal/admit"
	"gqladmit/internal/policy"
)

func newAdmitter() *admit.Admitter {
	return admit.New(
		admit.Limits{MaxDepth: 8, MaxFields: 200},
		policy.NewIntrospectionGuard(),
		policy.NewAuthorizer(
			[]string{"userSecret", "billingInfo"},
			map[string][]string{"acme": {"billingInfo"}},
		),
	)
}

func TestAdmitAcceptsOrdinaryQuery(t *testing.T) {
	a := newAdmitter()
	decision, err := a.Admit("acme", `
		query Dashboard {
			viewer {
				id
				name
				posts(first: 10) {
					edges { node { title } }
				}
			}
		}
	`)
	if err != nil {
		t.Fatalf("Admit returned error: %v", err)
	}
	if decision.Operation != "query Dashboard" {
		t.Errorf("operation = %q, want %q", decision.Operation, "query Dashboard")
	}
	if decision.Fields != 7 {
		t.Errorf("fields = %d, want 7", decision.Fields)
	}
	if decision.Depth != 5 {
		t.Errorf("depth = %d, want 5", decision.Depth)
	}
}

func TestAdmitAcceptsFragmentQuery(t *testing.T) {
	a := newAdmitter()
	if _, err := a.Admit("acme", `
		query {
			viewer { ...ProfileFields }
		}
		fragment ProfileFields on User {
			id
			name
			billingInfo { plan }
		}
	`); err != nil {
		t.Fatalf("Admit returned error: %v", err)
	}
}

func TestAdmitRejectsSyntaxError(t *testing.T) {
	a := newAdmitter()
	_, err := a.Admit("acme", `query { viewer { id }`)
	assertCode(t, err, admit.CodeSyntax)
}

func TestAdmitRejectsMultipleOperations(t *testing.T) {
	a := newAdmitter()
	_, err := a.Admit("acme", `query A { a } query B { b }`)
	assertCode(t, err, admit.CodeSyntax)
}

func TestAdmitRejectsDeepQuery(t *testing.T) {
	a := newAdmitter()
	_, err := a.Admit("acme", `
		{
			a { b { c { d { e { f { g { h { i } } } } } } } }
		}
	`)
	assertCode(t, err, admit.CodeDepth)
}

func TestAdmitRejectsWideQuery(t *testing.T) {
	var b strings.Builder
	b.WriteString("{ viewer {")
	for i := 0; i < 201; i++ {
		b.WriteString(" f")
		b.WriteString(strings.Repeat("x", i%3))
		b.WriteString(": id")
	}
	b.WriteString(" } }")
	_, err := newAdmitter().Admit("acme", b.String())
	assertCode(t, err, admit.CodeFields)
}

func TestAdmitRejectsIntrospection(t *testing.T) {
	a := newAdmitter()
	_, err := a.Admit("acme", `{ __schema { queryType { name } } }`)
	assertCode(t, err, admit.CodeIntrospection)
}

func TestAdmitRejectsRestrictedField(t *testing.T) {
	a := newAdmitter()
	_, err := a.Admit("acme", `{ viewer { userSecret } }`)
	assertCode(t, err, admit.CodeForbidden)
}

func TestAdmitRejectsUnknownFragment(t *testing.T) {
	a := newAdmitter()
	_, err := a.Admit("acme", `{ viewer { ...Missing } }`)
	assertCode(t, err, admit.CodeSyntax)
}

func assertCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("Admit returned no error, want code %s", want)
	}
	var rej *admit.Rejection
	if !errors.As(err, &rej) {
		t.Fatalf("error = %v, want *admit.Rejection", err)
	}
	if rej.Code != want {
		t.Errorf("rejection code = %q, want %q (message: %s)", rej.Code, want, rej.Message)
	}
}
