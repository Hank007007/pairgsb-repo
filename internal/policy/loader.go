package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// document is the on-disk shape of a policy file.
type document struct {
	Version     string             `json:"version"`
	Rules       []ruleDocument     `json:"rules"`
	Delegations []delegationRecord `json:"delegations"`
}

type ruleDocument struct {
	Name     string   `json:"name"`
	Roles    []string `json:"roles"`
	Resource string   `json:"resource"`
	Actions  []string `json:"actions"`
	Effect   string   `json:"effect"`
}

type delegationRecord struct {
	From     string   `json:"from"`
	To       string   `json:"to"`
	Resource string   `json:"resource"`
	Actions  []string `json:"actions"`
}

// LoadFile reads and parses the policy document at path.
func LoadFile(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read policy: %w", err)
	}
	return ParsePolicy(data)
}

// ParsePolicy validates the JSON encoding of a policy document.
func ParsePolicy(data []byte) (*Policy, error) {
	var doc document
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse policy: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("parse policy: unexpected trailing content")
	}

	p := &Policy{Version: doc.Version}
	for i, r := range doc.Rules {
		if r.Resource == "" {
			return nil, fmt.Errorf("rule %d: resource is required", i)
		}
		effect := Effect(r.Effect)
		if effect != Allow && effect != Deny {
			return nil, fmt.Errorf("rule %d: effect must be %q or %q", i, Allow, Deny)
		}
		name := r.Name
		if name == "" {
			name = fmt.Sprintf("rule-%d", i)
		}
		p.Rules = append(p.Rules, Rule{
			Name:     name,
			Roles:    r.Roles,
			Resource: r.Resource,
			Actions:  r.Actions,
			Effect:   effect,
		})
	}
	for i, d := range doc.Delegations {
		if d.From == "" || d.To == "" {
			return nil, fmt.Errorf("delegation %d: from and to are required", i)
		}
		if d.Resource == "" {
			return nil, fmt.Errorf("delegation %d: resource is required", i)
		}
		p.Delegations = append(p.Delegations, Delegation{
			From:     d.From,
			To:       d.To,
			Resource: d.Resource,
			Actions:  d.Actions,
		})
	}
	return p, nil
}
