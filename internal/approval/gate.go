// Package approval implements approval gate counting and role policy
// evaluation.
package approval

import (
	"errors"
	"fmt"
)

// Decision is a single approver's submitted decision.
type Decision string

const (
	// DecisionApprove records an approval.
	DecisionApprove Decision = "approve"
	// DecisionReject records a rejection.
	DecisionReject Decision = "reject"
)

// Vote is one submission from an approver.
type Vote struct {
	ApproverID string
	Role       string
	Decision   Decision
}

// Result describes the outcome of a gate evaluation.
type Result struct {
	Passed     bool
	Approvals  int
	Rejections int
}

// Gate evaluates whether enough approvals have been collected.
type Gate struct {
	Required int
}

// ErrInvalidRequired is returned when the gate configuration is unusable.
var ErrInvalidRequired = errors.New("approval: required approval count must be positive")

// Evaluate applies the gate to the supplied votes.
func (g Gate) Evaluate(votes []Vote) (Result, error) {
	if g.Required <= 0 {
		return Result{}, ErrInvalidRequired
	}

	var result Result
	for _, v := range votes {
		if v.ApproverID == "" {
			continue
		}
		switch v.Decision {
		case DecisionApprove:
			result.Approvals++
		case DecisionReject:
			result.Rejections++
		default:
			return Result{}, fmt.Errorf("approval: unknown decision %q", v.Decision)
		}
	}

	if result.Rejections > 0 {
		return result, nil
	}
	result.Passed = result.Approvals >= g.Required
	return result, nil
}
