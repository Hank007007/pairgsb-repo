package patch

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// Operation is one RFC 6902 patch operation.
type Operation struct {
	Op    string `json:"op"`
	Path  string `json:"path,omitempty"`
	From  string `json:"from,omitempty"`
	Value any    `json:"value,omitempty"`

	HasPath  bool `json:"-"`
	HasFrom  bool `json:"-"`
	HasValue bool `json:"-"`
}

// UnmarshalJSON decodes an operation, decoding `value` through the literal
// decoder used for JSON patch bodies.
func (o *Operation) UnmarshalJSON(data []byte) error {
	var raw struct {
		Op    string          `json:"op"`
		Path  *string         `json:"path"`
		From  *string         `json:"from"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	o.Op = raw.Op
	if raw.Path != nil {
		o.Path = *raw.Path
		o.HasPath = true
	}
	if raw.From != nil {
		o.From = *raw.From
		o.HasFrom = true
	}
	if raw.Value != nil {
		value, err := decodeLiteral(raw.Value)
		if err != nil {
			return err
		}
		o.Value = value
		o.HasValue = true
	}
	return nil
}

// ParseRequest decodes a JSON array of patch operations.
func ParseRequest(data []byte) ([]Operation, error) {
	var ops []Operation
	if err := json.Unmarshal(data, &ops); err != nil {
		return nil, err
	}
	return ops, nil
}

// decodeLiteral decodes a JSON literal, keeping integral numbers as ints.
func decodeLiteral(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	return normalizeNumbers(value), nil
}

func normalizeNumbers(value any) any {
	switch v := value.(type) {
	case json.Number:
		text := v.String()
		if !strings.ContainsAny(text, ".eE") {
			if n, err := strconv.ParseInt(text, 10, 64); err == nil {
				return int(n)
			}
		}
		f, err := v.Float64()
		if err != nil {
			return text
		}
		return f
	case []any:
		for i, item := range v {
			v[i] = normalizeNumbers(item)
		}
		return v
	case map[string]any:
		for key, item := range v {
			v[key] = normalizeNumbers(item)
		}
		return v
	default:
		return value
	}
}
