package approval

import "testing"

// TestGateCountsDistinctApprovers reproduces the review finding: one
// approver submitting three times still counts as a single vote.
func TestGateCountsDistinctApprovers(t *testing.T) {
	gate := Gate{Required: 3}
	result, err := gate.Evaluate([]Vote{
		{ApproverID: "alice", Role: "ops", Decision: DecisionApprove},
		{ApproverID: "alice", Role: "ops", Decision: DecisionApprove},
		{ApproverID: "alice", Role: "ops", Decision: DecisionApprove},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.Approvals != 1 {
		t.Fatalf("Approvals = %d, want 1 distinct approver", result.Approvals)
	}
	if result.Passed {
		t.Fatal("gate passed on three submissions from one approver")
	}
}

func TestGatePassesOnlyWithEnoughDistinctApprovers(t *testing.T) {
	gate := Gate{Required: 3}
	result, err := gate.Evaluate([]Vote{
		{ApproverID: "alice", Decision: DecisionApprove},
		{ApproverID: "alice", Decision: DecisionApprove},
		{ApproverID: "bob", Decision: DecisionApprove},
		{ApproverID: "carol", Decision: DecisionApprove},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.Approvals != 3 || !result.Passed {
		t.Fatalf("Approvals = %d, Passed = %v; want 3 distinct and passed", result.Approvals, result.Passed)
	}
}

func TestGateDeduplicatesRejections(t *testing.T) {
	gate := Gate{Required: 1}
	result, err := gate.Evaluate([]Vote{
		{ApproverID: "dave", Decision: DecisionReject},
		{ApproverID: "dave", Decision: DecisionReject},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.Rejections != 1 {
		t.Fatalf("Rejections = %d, want 1 distinct rejector", result.Rejections)
	}
	if result.Passed {
		t.Fatal("gate passed despite a rejection")
	}
}

func TestGateIgnoresEmptyApproverID(t *testing.T) {
	gate := Gate{Required: 1}
	result, err := gate.Evaluate([]Vote{
		{ApproverID: "", Decision: DecisionApprove},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.Approvals != 0 || result.Passed {
		t.Fatalf("empty approver-id counted: %+v", result)
	}
}
