package acl

import "testing"

func TestConnectionTableEmptyByDefault(t *testing.T) {
	table := NewConnectionTable()
	if table.Established(flow(ProtocolTCP, "10.0.1.10", "10.0.2.20", 51000, 5432)) {
		t.Fatal("an empty connection table reported a flow as established")
	}
}

func TestConnectionTableReset(t *testing.T) {
	table := NewConnectionTable()
	table.Establish(flow(ProtocolTCP, "10.0.1.10", "10.0.2.20", 51000, 5432))
	table.Reset()
	if table.Established(flow(ProtocolTCP, "10.0.1.10", "10.0.2.20", 51000, 5432)) {
		t.Fatal("Reset left a connection behind")
	}
}

func TestEvaluateNewConnectionUsesRuleBook(t *testing.T) {
	group := SecurityGroup{ID: "sg-db", Ingress: NewRuleBook()}
	ev := NewEvaluator(registryWith(group))

	decision, err := ev.Evaluate(Query{
		GroupID:   "sg-db",
		Direction: Ingress,
		Flow:      flow(ProtocolTCP, "10.0.1.10", "10.0.2.20", 51000, 5432),
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if decision.Stateful {
		t.Fatal("a new connection was reported as stateful")
	}
	if decision.Allowed {
		t.Fatal("the connection table admitted a flow that was never established")
	}
}
