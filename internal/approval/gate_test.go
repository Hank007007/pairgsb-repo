package approval

import "testing"

func TestGatePassesWithDistinctApprovers(t *testing.T) {
	gate := Gate{Required: 2}
	result, err := gate.Evaluate([]Vote{
		{ApproverID: "alice", Role: "ops", Decision: DecisionApprove},
		{ApproverID: "bob", Role: "ops", Decision: DecisionApprove},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !result.Passed {
		t.Fatal("gate did not pass with two distinct approvals")
	}
}

func TestGateFailsBelowThreshold(t *testing.T) {
	gate := Gate{Required: 3}
	result, err := gate.Evaluate([]Vote{
		{ApproverID: "alice", Role: "admin", Decision: DecisionApprove},
		{ApproverID: "bob", Role: "admin", Decision: DecisionApprove},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.Passed {
		t.Fatal("gate passed below the required approval count")
	}
}

func TestGateRejectsOnAnyRejection(t *testing.T) {
	gate := Gate{Required: 2}
	result, err := gate.Evaluate([]Vote{
		{ApproverID: "alice", Role: "admin", Decision: DecisionApprove},
		{ApproverID: "bob", Role: "admin", Decision: DecisionApprove},
		{ApproverID: "carol", Role: "admin", Decision: DecisionReject},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.Passed {
		t.Fatal("gate passed despite a rejection")
	}
}

func TestGateRejectsUnknownDecision(t *testing.T) {
	gate := Gate{Required: 1}
	if _, err := gate.Evaluate([]Vote{
		{ApproverID: "alice", Role: "admin", Decision: Decision("maybe")},
	}); err == nil {
		t.Fatal("Evaluate accepted an unknown decision")
	}
}

func TestGateRejectsInvalidConfiguration(t *testing.T) {
	if _, err := (Gate{}).Evaluate(nil); err == nil {
		t.Fatal("Evaluate accepted a zero required count")
	}
}
