package broker_test

import (
	"os"
	"testing"

	"topicwarden/internal/audit"
	"topicwarden/internal/broker"
)

func newAuthorizer(t *testing.T) *broker.Authorizer {
	t.Helper()
	data, err := os.ReadFile("../../testdata/rules.yaml")
	if err != nil {
		t.Fatalf("read rules: %v", err)
	}
	log := audit.NewLogger(nil)
	a := broker.NewAuthorizer(nil, log)
	if err := a.Reload(data); err != nil {
		t.Fatalf("reload: %v", err)
	}
	return a
}

func TestSubscribePublishDeliver(t *testing.T) {
	a := newAuthorizer(t)
	conn := &broker.Connection{ClientID: "c-1", PrincipalClass: "sensor"}

	sub, ok := a.Subscribe(conn, "sensors/+/temp")
	if !ok {
		t.Fatalf("expected subscription to be allowed")
	}

	if !a.AuthorizePublish(conn, "sensors/a/temp") {
		t.Fatalf("expected publish to be allowed")
	}

	if !a.Deliver(sub, "sensors/a/temp") {
		t.Fatalf("expected delivery to be allowed")
	}

	if a.AuthorizePublish(conn, "other/a/temp") {
		t.Fatalf("expected publish outside the rule to be denied")
	}
}
