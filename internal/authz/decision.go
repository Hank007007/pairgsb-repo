package authz

// Binding is a role binding in the form the evaluator consumes. Namespace is
// empty for cluster-scoped bindings.
type Binding struct {
	Namespace string
	Subjects  []Subject
	Rules     []Rule
}

// Subject is one grantee of a binding.
type Subject struct {
	Kind string
	Name string
}

// Identity is the effective subject of a review after impersonation and group
// expansion have been applied.
type Identity struct {
	User   string
	Groups []string
}

// Result carries the decision together with an audit reason.
type Result struct {
	Decision Decision
	Reason   string
}

// Evaluate scans the bindings that apply to the request's namespace and
// produces the combined decision.
func Evaluate(req Request, identity Identity, bindings []Binding) Result {
	allowReason := ""
	denyReason := ""

	for _, binding := range bindings {
		if !bindingApplies(binding, req) {
			continue
		}
		if !bindingMatchesIdentity(binding, identity) {
			continue
		}
		for _, rule := range binding.Rules {
			if !rule.Matches(req) {
				continue
			}
			switch rule.effectiveEffect() {
			case EffectDeny:
				if denyReason == "" {
					denyReason = "rule denies " + req.Verb + " on " + req.FullResource()
				}
			case EffectAllow:
				if allowReason == "" {
					allowReason = "rule allows " + req.Verb + " on " + req.FullResource()
				}
			}
		}
	}

	// Deny wins regardless of evaluation order (§5.2, §5.5).
	if denyReason != "" {
		return Result{Decision: DecisionDeny, Reason: denyReason}
	}
	if allowReason != "" {
		return Result{Decision: DecisionAllow, Reason: allowReason}
	}
	return Result{Decision: DecisionNoOpinion, Reason: "no matching rule"}
}

func (r Rule) effectiveEffect() Effect {
	if r.Effect == "" {
		return EffectAllow
	}
	return r.Effect
}

func bindingApplies(binding Binding, req Request) bool {
	if binding.Namespace == "" {
		return true
	}
	return binding.Namespace == req.Namespace
}

func bindingMatchesIdentity(binding Binding, identity Identity) bool {
	for _, subject := range binding.Subjects {
		switch subject.Kind {
		case "User":
			if subject.Name == identity.User {
				return true
			}
		case "Group":
			for _, group := range identity.Groups {
				if group == subject.Name {
					return true
				}
			}
		}
	}
	return false
}
