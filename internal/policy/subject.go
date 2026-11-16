package policy

// ActionAny is the action token that stands for every action in a policy document.
const ActionAny = "*"

// Subject is the caller identity presented to the decision point: the workload's service
// account, the role chain the platform issued to it, and the version of that chain.
type Subject struct {
	ServiceAccount string
	Roles          []string
	RoleVersion    string
}

// hasRole reports whether the subject's role chain contains name.
func (s Subject) hasRole(name string) bool {
	for _, r := range s.Roles {
		if r == name {
			return true
		}
	}
	return false
}

// Request is a single authorization question.
type Request struct {
	Subject  Subject
	Resource string
	Action   string
}

// Effect is the outcome of an authorization question.
type Effect string

const (
	Allow Effect = "allow"
	Deny  Effect = "deny"
)

// Decision is the answer produced for a Request.
type Decision struct {
	Effect Effect
	Reason string
	Source string
}

// Allowed reports whether the decision permits the request.
func (d Decision) Allowed() bool {
	return d.Effect == Allow
}

// Rule is one entry of a policy document.
type Rule struct {
	Name     string
	Roles    []string
	Resource string
	Actions  []string
	Effect   Effect
}

// Delegation is a directed grant from one role to another, scoped to a resource pattern and
// an action set.
type Delegation struct {
	From     string
	To       string
	Resource string
	Actions  []string
}

// Policy is a loaded policy document.
type Policy struct {
	Version     string
	Rules       []Rule
	Delegations []Delegation
}

// actionCovered reports whether an action list covers action. An empty list covers every
// action, as does the ActionAny token.
func actionCovered(actions []string, action string) bool {
	if len(actions) == 0 {
		return true
	}
	for _, a := range actions {
		if a == ActionAny || a == action {
			return true
		}
	}
	return false
}
