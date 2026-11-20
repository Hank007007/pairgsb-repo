// Package engine exposes the decision point used by the API gateway.
package engine

import (
	"fmt"

	"condgrate/internal/attr"
	"condgrate/internal/cond"
	"condgrate/internal/policy"
)

// Request is the input to a single authorization decision.
type Request struct {
	Principal map[string]any
	Resource  map[string]any
	Context   map[string]any
	Action    string
}

// Response is the outcome returned to the caller.
type Response struct {
	Effect        policy.Effect
	Obligations   []string
	Matched       []string
	Reason        string
	Indeterminate bool
}

// Engine evaluates requests against a policy chain.
type Engine struct {
	Chain     *policy.Chain
	Directory *attr.Directory
}

// New builds an engine over a chain and a directory snapshot.
func New(chain *policy.Chain, dir *attr.Directory) *Engine {
	return &Engine{Chain: chain, Directory: dir}
}

// Decide evaluates one request and returns the merged decision.
func (e *Engine) Decide(req Request) Response {
	hints := &attr.RequestAttributes{
		Principal: req.Principal,
		Resource:  req.Resource,
		Context:   req.Context,
	}
	store := attr.NewStore(e.Directory, hints, req.Action)

	match := policy.MatchRequest{
		Principal: policy.BuildBlock(e.effective("principal", req.Principal)),
		Resource:  policy.BuildBlock(e.effective("resource", req.Resource)),
		Action:    req.Action,
	}

	candidates := policy.Collect(e.Chain, match, store)

	var reasons []string
	for _, c := range candidates {
		if c.Verdict == cond.Indeterminate {
			reasons = append(reasons, fmt.Sprintf("policy %s: condition indeterminate", c.Policy.ID))
		}
	}

	decision := policy.Combine(candidates)
	resp := Response{
		Effect:      decision.Effect,
		Obligations: decision.Obligations,
		Matched:     decision.Matched,
	}
	if len(decision.Reasons) > 0 {
		resp.Reason = decision.Reasons[0]
	}
	if len(reasons) > 0 {
		resp.Effect = policy.Deny
		resp.Obligations = nil
		resp.Indeterminate = true
		resp.Reason = joinReasons(reasons)
	}
	return resp
}

func (e *Engine) effective(source string, provided map[string]any) map[string]any {
	out := make(map[string]any, len(provided))
	for k, v := range provided {
		out[k] = v
	}
	if e.Directory != nil {
		for k, v := range e.Directory.Values(source) {
			out[k] = v
		}
	}
	return out
}

func joinReasons(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	out := reasons[0]
	for _, r := range reasons[1:] {
		out += "; " + r
	}
	return out
}
