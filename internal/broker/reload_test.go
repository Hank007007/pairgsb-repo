package broker_test

import (
	"testing"

	"topicwarden/internal/audit"
	"topicwarden/internal/broker"
)

func TestReloadInstallsNewRules(t *testing.T) {
	a := newAuthorizer(t)
	conn := &broker.Connection{ClientID: "c-1", PrincipalClass: "sensor"}

	if !a.AuthorizePublish(conn, "sensors/a/temp") {
		t.Fatalf("publish should be allowed before reload")
	}

	err := a.Reload([]byte(`
rules:
  - id: other
    effect: allow
    principal: sensor
    action: pub
    filter: other/#
`))
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := a.Store().Version(); got != 2 {
		t.Fatalf("version = %d, want 2", got)
	}
}

func TestReloadRejectsMalformedDocument(t *testing.T) {
	a := newAuthorizer(t)
	if err := a.Reload([]byte("rules: [ not: valid")); err == nil {
		t.Fatalf("expected malformed document to be rejected")
	}
	if got := a.Store().Version(); got != 1 {
		t.Fatalf("version changed after failed reload: %d", got)
	}
}

func TestDenyIsAudited(t *testing.T) {
	log := audit.NewLogger(nil)
	a := broker.NewAuthorizer(nil, log)
	if err := a.Reload([]byte(`
rules:
  - id: sensors-read
    effect: allow
    principal: sensor
    action: sub
    filter: sensors/+/temp
`)); err != nil {
		t.Fatalf("reload: %v", err)
	}

	conn := &broker.Connection{ClientID: "c-1", PrincipalClass: "sensor"}
	if a.AuthorizeSubscribe(conn, "secret/#") {
		t.Fatalf("expected denial")
	}

	events := log.Events()
	if len(events) != 1 {
		t.Fatalf("got %d audit events, want 1", len(events))
	}
	if events[0].Decision != audit.DecisionDeny {
		t.Fatalf("decision = %q, want deny", events[0].Decision)
	}
}
