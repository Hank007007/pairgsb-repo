// Package policy loads, collects and combines authorization policies.
package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"condgrate/internal/cond"
)

// Effect is the outcome a policy grants.
type Effect string

const (
	// Permit allows the request.
	Permit Effect = "permit"
	// Deny blocks the request.
	Deny Effect = "deny"
)

// MatchBlock maps an attribute name to the candidate values accepted for it.
// A candidate may be a scalar or a list; both are normalised to strings.
type MatchBlock map[string][]string

// Policy is one authorization policy document.
type Policy struct {
	ID          string     `json:"id"`
	Effect      Effect     `json:"effect"`
	Principal   MatchBlock `json:"principal"`
	Action      []string   `json:"action"`
	Resource    MatchBlock `json:"resource"`
	Condition   *cond.Node `json:"condition,omitempty"`
	Obligations []string   `json:"obligations,omitempty"`
}

// Layer names the scope a policy is attached to.
type Layer string

const (
	// OrgLayer is the topmost scope.
	OrgLayer Layer = "org"
	// TeamLayer is the middle scope.
	TeamLayer Layer = "team"
	// ProjectLayer is the most specific scope.
	ProjectLayer Layer = "project"
)

// Target is the match block attached to one layer and inherited by policies
// hanging below it.
type Target struct {
	Principal MatchBlock `json:"principal"`
	Action    []string   `json:"action"`
	Resource  MatchBlock `json:"resource"`
}

// Scope groups the policies mounted at one layer of the hierarchy.
type Scope struct {
	Layer    Layer    `json:"layer"`
	Name     string   `json:"name"`
	Target   Target   `json:"target"`
	Policies []Policy `json:"policies"`
}

// Chain is the ordered hierarchy org → team → project.
type Chain struct {
	Org     Scope
	Team    Scope
	Project Scope
}

// LoadChain reads a chain from a JSON file.
func LoadChain(path string) (*Chain, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read chain: %w", err)
	}
	return ParseChain(data)
}

// ParseChain decodes a chain from JSON and validates it.
func ParseChain(data []byte) (*Chain, error) {
	var raw struct {
		Org     Scope `json:"org"`
		Team    Scope `json:"team"`
		Project Scope `json:"project"`
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode chain: %w", err)
	}
	raw.Org.Layer = OrgLayer
	raw.Team.Layer = TeamLayer
	raw.Project.Layer = ProjectLayer
	chain := &Chain{Org: raw.Org, Team: raw.Team, Project: raw.Project}
	if err := chain.Validate(); err != nil {
		return nil, err
	}
	return chain, nil
}

// Validate checks that effects and identifiers are well formed.
func (c *Chain) Validate() error {
	seen := make(map[string]bool)
	for _, scope := range []Scope{c.Org, c.Team, c.Project} {
		for _, p := range scope.Policies {
			if p.ID == "" {
				return fmt.Errorf("%s policy has no id", scope.Layer)
			}
			if seen[p.ID] {
				return fmt.Errorf("duplicate policy id %q", p.ID)
			}
			seen[p.ID] = true
			if p.Effect != Permit && p.Effect != Deny {
				return fmt.Errorf("policy %q has invalid effect %q", p.ID, p.Effect)
			}
		}
	}
	return nil
}

// MatchBlock unmarshals from either a scalar, a list or a nested list.
func (m *MatchBlock) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	out := make(MatchBlock, len(raw))
	for key, val := range raw {
		var one any
		if err := json.Unmarshal(val, &one); err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		out[key] = flatten(one)
	}
	*m = out
	return nil
}

// BuildBlock converts a request attribute map into a MatchBlock.
func BuildBlock(values map[string]any) MatchBlock {
	out := make(MatchBlock, len(values))
	for k, v := range values {
		out[k] = flatten(v)
	}
	return out
}

func flatten(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			out = append(out, flatten(item)...)
		}
		return out
	case nil:
		return nil
	default:
		return []string{fmt.Sprint(t)}
	}
}
