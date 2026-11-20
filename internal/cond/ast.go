// Package cond defines the condition expression tree used by policies.
package cond

import (
	"encoding/json"
	"fmt"
)

// Node is a single condition expression. Exactly one field is populated for a
// well-formed node; a zero Node is treated as an empty expression.
type Node struct {
	And      []Node
	Or       []Node
	Not      *Node
	Cmp      *Cmp
	In       *In
	Contains *Contains
	Present  *Present
}

// Cmp is an ordered comparison between two references.
type Cmp struct {
	Op    string
	Left  Ref
	Right Ref
}

// In tests whether the value at Path is a member of Values.
type In struct {
	Path   string
	Values []Ref
}

// Contains tests whether Path holds Value.
type Contains struct {
	Path  string
	Value Ref
}

// Present tests whether Path exists.
type Present struct {
	Path string
}

// Ref is either an attribute reference or a literal JSON value.
type Ref struct {
	IsAttr  bool
	Attr    string
	Literal any
}

// UnmarshalJSON decodes a condition node from its JSON form.
func (n *Node) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("condition node must be an object: %w", err)
	}
	if len(raw) == 0 {
		return fmt.Errorf("condition node is empty")
	}
	for key, val := range raw {
		switch key {
		case "and":
			var list []Node
			if err := json.Unmarshal(val, &list); err != nil {
				return fmt.Errorf("and: %w", err)
			}
			n.And = list
			n.Or, n.Not, n.Cmp, n.In, n.Contains, n.Present = nil, nil, nil, nil, nil, nil
		case "or":
			var list []Node
			if err := json.Unmarshal(val, &list); err != nil {
				return fmt.Errorf("or: %w", err)
			}
			n.Or = list
			n.And, n.Not, n.Cmp, n.In, n.Contains, n.Present = nil, nil, nil, nil, nil, nil
		case "not":
			var child Node
			if err := json.Unmarshal(val, &child); err != nil {
				return fmt.Errorf("not: %w", err)
			}
			n.Not = &child
			n.And, n.Or, n.Cmp, n.In, n.Contains, n.Present = nil, nil, nil, nil, nil, nil
		case "cmp":
			var c Cmp
			if err := json.Unmarshal(val, &c); err != nil {
				return fmt.Errorf("cmp: %w", err)
			}
			n.Cmp = &c
			n.And, n.Or, n.Not, n.In, n.Contains, n.Present = nil, nil, nil, nil, nil, nil
		case "in":
			var raw struct {
				Path   string `json:"path"`
				Values []Ref  `json:"values"`
			}
			if err := json.Unmarshal(val, &raw); err != nil {
				return fmt.Errorf("in: %w", err)
			}
			n.In = &In{Path: raw.Path, Values: raw.Values}
			n.And, n.Or, n.Not, n.Cmp, n.Contains, n.Present = nil, nil, nil, nil, nil, nil
		case "contains":
			var raw struct {
				Path  string `json:"path"`
				Value Ref    `json:"value"`
			}
			if err := json.Unmarshal(val, &raw); err != nil {
				return fmt.Errorf("contains: %w", err)
			}
			n.Contains = &Contains{Path: raw.Path, Value: raw.Value}
			n.And, n.Or, n.Not, n.Cmp, n.In, n.Present = nil, nil, nil, nil, nil, nil
		case "present":
			var raw struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(val, &raw); err != nil {
				return fmt.Errorf("present: %w", err)
			}
			n.Present = &Present{Path: raw.Path}
			n.And, n.Or, n.Not, n.Cmp, n.In, n.Contains = nil, nil, nil, nil, nil, nil
		default:
			return fmt.Errorf("unknown condition node %q", key)
		}
	}
	return nil
}

// UnmarshalJSON decodes an attribute reference or a literal value.
func (r *Ref) UnmarshalJSON(data []byte) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err == nil {
		if attr, ok := obj["attr"]; ok && len(obj) == 1 {
			var path string
			if err := json.Unmarshal(attr, &path); err != nil {
				return fmt.Errorf("attr: %w", err)
			}
			r.IsAttr = true
			r.Attr = path
			r.Literal = nil
			return nil
		}
	}
	var lit any
	if err := json.Unmarshal(data, &lit); err != nil {
		return fmt.Errorf("ref: %w", err)
	}
	r.IsAttr = false
	r.Attr = ""
	r.Literal = lit
	return nil
}
