package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"scopewall/internal/cache"
	"scopewall/internal/policy"
)

// Header names understood by the middleware.
const (
	HeaderServiceAccount = "X-Scope-Service-Account"
	HeaderRoles          = "X-Scope-Roles"
	HeaderRoleVersion    = "X-Scope-Role-Version"
	HeaderAction         = "X-Scope-Action"
	HeaderDecision       = "X-Scope-Decision"
)

// Authorizer is the HTTP decision point. It builds a request from the inbound headers and
// path, consults the cache and the policy engine, and forwards allowed requests.
type Authorizer struct {
	Next   http.Handler
	Source *Source
	Cache  *cache.DecisionCache
}

// RequestFromHTTP builds the authorization question carried by r.
func RequestFromHTTP(r *http.Request) policy.Request {
	action := r.Header.Get(HeaderAction)
	if action == "" {
		action = ActionForMethod(r.Method)
	}
	return policy.Request{
		Subject: policy.Subject{
			ServiceAccount: r.Header.Get(HeaderServiceAccount),
			Roles:          SplitRoles(r.Header.Get(HeaderRoles)),
			RoleVersion:    r.Header.Get(HeaderRoleVersion),
		},
		Resource: r.URL.Path,
		Action:   action,
	}
}

// SplitRoles parses a comma-separated role chain.
func SplitRoles(raw string) []string {
	var roles []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			roles = append(roles, part)
		}
	}
	return roles
}

// ActionForMethod maps an HTTP method to the action token used in policies when the caller
// does not name one explicitly.
func ActionForMethod(method string) string {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead:
		return "read"
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return "write"
	case http.MethodDelete:
		return "delete"
	default:
		return "read"
	}
}

// ServeHTTP authorizes the request and either forwards it or rejects it.
func (a *Authorizer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req := RequestFromHTTP(r)

	p, err := a.Source.Policy()
	if err != nil {
		// Without a policy in force, requests are left to the downstream service.
		a.Next.ServeHTTP(w, r)
		return
	}

	key := cache.NewKey(req)
	if decision, ok := a.Cache.Get(key); ok {
		a.finish(w, r, decision)
		return
	}

	decision := policy.NewEngine(p).Evaluate(req)
	if decision.Reason == policy.ReasonNoMatch {
		// Nothing in the policy speaks to this request, so it is passed along.
		a.Next.ServeHTTP(w, r)
		return
	}

	a.Cache.Put(key, decision)
	a.finish(w, r, decision)
}

func (a *Authorizer) finish(w http.ResponseWriter, r *http.Request, decision policy.Decision) {
	w.Header().Set(HeaderDecision, decision.Source)
	if decision.Allowed() {
		a.Next.ServeHTTP(w, r)
		return
	}
	writeDenial(w, decision)
}

type denialBody struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
	Source   string `json:"source"`
}

func writeDenial(w http.ResponseWriter, decision policy.Decision) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(denialBody{
		Decision: string(decision.Effect),
		Reason:   decision.Reason,
		Source:   decision.Source,
	})
}
