package authz

// Effect selects what a matching rule does to the overall decision.
type Effect string

const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

// Rule is one entry of a Role or ClusterRole.
type Rule struct {
	Effect        Effect   `json:"effect,omitempty"`
	Verbs         []string `json:"verbs"`
	APIGroups     []string `json:"apiGroups"`
	Resources     []string `json:"resources"`
	ResourceNames []string `json:"resourceNames,omitempty"`
}

// Request is the normalized form of a SubjectAccessReview used during
// matching. Subresource is kept separate from Resource so that callers cannot
// accidentally lose the distinction between the two.
type Request struct {
	Verb        string
	APIGroup    string
	Resource    string
	Subresource string
	Namespace   string
	Name        string
}

// RequestFromReview flattens a SubjectAccessReview into a Request.
func RequestFromReview(review SubjectAccessReview) Request {
	return Request{
		Verb:        review.Verb,
		APIGroup:    review.APIGroup,
		Resource:    review.Resource,
		Subresource: review.Subresource,
		Namespace:   review.Namespace,
		Name:        review.Name,
	}
}

// FullResource is the resource as written in a rule: "pods" or "pods/exec".
func (r Request) FullResource() string {
	if r.Subresource == "" {
		return r.Resource
	}
	return r.Resource + "/" + r.Subresource
}

// Matches reports whether every field of the rule accepts the request.
func (r Rule) Matches(req Request) bool {
	return matchVerb(r, req) &&
		matchAPIGroup(r, req) &&
		matchResource(r, req) &&
		matchName(r, req)
}

func matchVerb(rule Rule, req Request) bool {
	for _, verb := range rule.Verbs {
		if verb == "*" || verb == req.Verb {
			return true
		}
	}
	return false
}

func matchAPIGroup(rule Rule, req Request) bool {
	for _, group := range rule.APIGroups {
		if group == "*" || group == req.APIGroup {
			return true
		}
	}
	return false
}

func matchResource(rule Rule, req Request) bool {
	full := req.FullResource()
	for _, resource := range rule.Resources {
		// The wildcard covers every resource identifier, including
		// subresources such as "pods/exec" (docs/authz-semantics.md §3.2).
		if resource == "*" || resource == full {
			return true
		}
	}
	return false
}

func matchName(rule Rule, req Request) bool {
	if len(rule.ResourceNames) == 0 {
		return true
	}
	// A rule naming concrete objects never matches collection-level
	// requests that carry no object name (§4.2, §4.4).
	if req.Name == "" {
		return false
	}
	for _, name := range rule.ResourceNames {
		if name == "*" || name == req.Name {
			return true
		}
	}
	return false
}
