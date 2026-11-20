package authz

import "fmt"

// Impersonation describes an identity the caller is acting as. When present it
// replaces the requesting user and group list for authorization purposes.
type Impersonation struct {
	User   string   `json:"user"`
	Groups []string `json:"groups,omitempty"`
}

// SubjectAccessReview is the payload the API server sends for every request it
// wants authorized. Exactly one verb, API group, resource, namespace and name
// is described per review.
type SubjectAccessReview struct {
	User          string         `json:"user"`
	Groups        []string       `json:"groups,omitempty"`
	Verb          string         `json:"verb"`
	Namespace     string         `json:"namespace,omitempty"`
	APIGroup      string         `json:"apiGroup,omitempty"`
	Resource      string         `json:"resource"`
	Subresource   string         `json:"subresource,omitempty"`
	Name          string         `json:"name,omitempty"`
	Impersonation *Impersonation `json:"impersonation,omitempty"`
}

// ReviewResponse is returned to the API server.
type ReviewResponse struct {
	Allowed bool   `json:"allowed"`
	Denied  bool   `json:"denied"`
	Reason  string `json:"reason,omitempty"`
}

// Decision is the outcome of evaluating one review.
type Decision string

const (
	DecisionAllow     Decision = "allow"
	DecisionDeny      Decision = "deny"
	DecisionNoOpinion Decision = "no-opinion"
)

func (d Decision) String() string {
	return string(d)
}

func ResponseFor(d Decision, reason string) ReviewResponse {
	switch d {
	case DecisionAllow:
		return ReviewResponse{Allowed: true, Reason: reason}
	case DecisionDeny:
		return ReviewResponse{Denied: true, Reason: reason}
	default:
		return ReviewResponse{Reason: reason}
	}
}

func (d Decision) Validate() error {
	switch d {
	case DecisionAllow, DecisionDeny, DecisionNoOpinion:
		return nil
	default:
		return fmt.Errorf("unknown decision %q", string(d))
	}
}
