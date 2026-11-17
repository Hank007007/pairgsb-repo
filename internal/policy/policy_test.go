package policy_test

import (
	"strings"
	"testing"

	"gqladmit/internal/graphql"
	"gqladmit/internal/parse"
	"gqladmit/internal/policy"
)

func parseSelections(t *testing.T, src string) []graphql.Selection {
	t.Helper()
	doc, err := parse.Parse(src)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	expanded, err := parse.Expand(doc)
	if err != nil {
		t.Fatalf("Expand returned error: %v", err)
	}
	return expanded.Operations[0].SelectionSet
}

func TestIntrospectionGuardAllowsTypename(t *testing.T) {
	guard := policy.NewIntrospectionGuard()
	sels := parseSelections(t, `{ viewer { __typename id } }`)
	if err := guard.Check(sels); err != nil {
		t.Errorf("Check returned error for __typename: %v", err)
	}
}

func TestIntrospectionGuardRejectsSchema(t *testing.T) {
	guard := policy.NewIntrospectionGuard()
	sels := parseSelections(t, `{ __schema { types { name } } }`)
	err := guard.Check(sels)
	if err == nil {
		t.Fatal("Check accepted __schema, want an error")
	}
	if !strings.Contains(err.Error(), "__schema") {
		t.Errorf("error = %q, want it to name __schema", err)
	}
}

func TestIntrospectionGuardRejectsType(t *testing.T) {
	guard := policy.NewIntrospectionGuard()
	sels := parseSelections(t, `{ __type(name: "User") { fields { name } } }`)
	if err := guard.Check(sels); err == nil {
		t.Fatal("Check accepted __type, want an error")
	}
}

func TestIntrospectionGuardAllowsOrdinaryUnderscoreFields(t *testing.T) {
	guard := policy.NewIntrospectionGuard()
	sels := parseSelections(t, `{ viewer { _id internal_code } }`)
	if err := guard.Check(sels); err != nil {
		t.Errorf("Check returned error for underscore-prefixed fields: %v", err)
	}
}

func newTestAuthorizer() *policy.Authorizer {
	return policy.NewAuthorizer(
		[]string{"userSecret", "billingInfo"},
		map[string][]string{
			"acme":   {"billingInfo"},
			"globex": {"userSecret", "billingInfo"},
		},
	)
}

func TestAuthorizerAllowsPublicFields(t *testing.T) {
	a := newTestAuthorizer()
	sels := parseSelections(t, `{ viewer { id name email } }`)
	if err := a.Check("acme", sels); err != nil {
		t.Errorf("Check returned error for public fields: %v", err)
	}
}

func TestAuthorizerAllowsGrantedField(t *testing.T) {
	a := newTestAuthorizer()
	sels := parseSelections(t, `{ viewer { billingInfo { plan } } }`)
	if err := a.Check("acme", sels); err != nil {
		t.Errorf("Check returned error for granted field: %v", err)
	}
}

func TestAuthorizerRejectsRestrictedField(t *testing.T) {
	a := newTestAuthorizer()
	sels := parseSelections(t, `{ viewer { userSecret } }`)
	err := a.Check("acme", sels)
	if err == nil {
		t.Fatal("Check accepted userSecret for acme, want an error")
	}
	if !strings.Contains(err.Error(), "userSecret") {
		t.Errorf("error = %q, want it to name userSecret", err)
	}
}

func TestAuthorizerRejectsRestrictedFieldInFragment(t *testing.T) {
	a := newTestAuthorizer()
	sels := parseSelections(t, `
		{ viewer { ...Sensitive } }
		fragment Sensitive on User { billingInfo { cardLast4 } }
	`)
	if err := a.Check("initech", sels); err == nil {
		t.Fatal("Check accepted billingInfo for initech, want an error")
	}
}
