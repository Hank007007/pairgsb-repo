// Package gate turns a submitted request into typed values for downstream
// consumers, or into the field errors that become a 400 response.
package gate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"strictcast/internal/shape"
)

// Result holds the typed values accepted for one request. Values are keyed by
// the registered field path.
type Result struct {
	values map[string]any
}

// Value returns the bound value stored under path.
func (r *Result) Value(path string) (any, bool) {
	v, ok := r.values[path]
	return v, ok
}

// String returns a bound string field.
func (r *Result) String(path string) (string, bool) {
	v, ok := r.values[path].(string)
	return v, ok
}

// Int returns a bound integer field.
func (r *Result) Int(path string) (int32, bool) {
	v, ok := r.values[path].(int32)
	return v, ok
}

// Bool returns a bound boolean field.
func (r *Result) Bool(path string) (bool, bool) {
	v, ok := r.values[path].(bool)
	return v, ok
}

// Strings returns a bound list field.
func (r *Result) Strings(path string) ([]string, bool) {
	v, ok := r.values[path].([]string)
	return v, ok
}

// Values returns a copy of the bound values.
func (r *Result) Values() map[string]any {
	out := make(map[string]any, len(r.values))
	for k, v := range r.values {
		out[k] = v
	}
	return out
}

// MarshalJSON renders the result as a JSON object of field path to value.
func (r *Result) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(r.values); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// FieldError describes one rejected field. Path is the registered (or
// submitted) field path; Code is the stable error code.
type FieldError struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

// BindError is the 400 payload for a rejected request.
type BindError struct {
	Errors []FieldError `json:"errors"`
}

func (e *BindError) Error() string {
	if len(e.Errors) == 0 {
		return "gate: request rejected"
	}
	return fmt.Sprintf("gate: %s at %s", e.Errors[0].Code, e.Errors[0].Path)
}

// Bind resolves entries against sh and coerces every registered field.
//
// Structural problems (unparseable keys, repeated or ambiguous paths, nesting
// beyond the shape limit) reject the request before any conversion. Field
// problems are collected per path and all returned together.
func Bind(sh *shape.Shape, entries []shape.Entry) (*Result, *BindError) {
	if sh == nil {
		return nil, &BindError{Errors: []FieldError{{Path: "", Code: shape.CodeBadPath}}}
	}
	tree, err := shape.Expand(entries, sh.MaxDepth)
	if err != nil {
		var ee *shape.ExpandError
		if ok := asExpandError(err, &ee); ok {
			return nil, &BindError{Errors: []FieldError{{Path: ee.Path, Code: ee.Code}}}
		}
		return nil, &BindError{Errors: []FieldError{{Path: "", Code: shape.CodeBadPath}}}
	}

	var errs []FieldError
	values := map[string]any{}
	for _, name := range sh.FieldNames() {
		field := sh.Fields[name]
		node, ok := shape.Lookup(tree, name)
		if !ok {
			if field.Required {
				errs = append(errs, FieldError{Path: name, Code: shape.CodeMissing})
			}
			continue
		}
		switch field.Kind {
		case shape.KindInt:
			raw, ok := scalar(node)
			if !ok {
				errs = append(errs, invalid(name, shape.CodeInvalidInt, ""))
				continue
			}
			n, err := shape.CoerceInt(raw)
			if err != nil {
				errs = append(errs, invalid(name, intCode(err), raw))
				continue
			}
			values[name] = n
		case shape.KindBool:
			raw, ok := scalar(node)
			if !ok {
				errs = append(errs, invalid(name, shape.CodeInvalidBool, ""))
				continue
			}
			b, err := shape.CoerceBool(raw)
			if err != nil {
				errs = append(errs, invalid(name, shape.CodeInvalidBool, raw))
				continue
			}
			values[name] = b
		case shape.KindString:
			raw, ok := scalar(node)
			if !ok {
				errs = append(errs, invalid(name, shape.CodeInvalidValue, ""))
				continue
			}
			s, err := shape.CoerceString(raw, field)
			if err != nil {
				errs = append(errs, invalid(name, stringCode(err), raw))
				continue
			}
			values[name] = s
		case shape.KindStringList:
			items, ok := list(node)
			if !ok {
				errs = append(errs, invalid(name, shape.CodeInvalidValue, ""))
				continue
			}
			out := make([]string, 0, len(items))
			for i, raw := range items {
				s, err := shape.CoerceString(raw, field)
				if err != nil {
					elem := fmt.Sprintf("%s[%d]", name, i)
					errs = append(errs, invalid(elem, stringCode(err), raw))
					continue
				}
				out = append(out, s)
			}
			values[name] = out
		default:
			errs = append(errs, invalid(name, shape.CodeInvalidValue, ""))
		}
	}

	if len(errs) > 0 {
		sort.SliceStable(errs, func(i, j int) bool {
			if errs[i].Path == errs[j].Path {
				return errs[i].Code < errs[j].Code
			}
			return errs[i].Path < errs[j].Path
		})
		return nil, &BindError{Errors: errs}
	}
	return &Result{values: values}, nil
}

// invalid builds the field error for a rejected value.
func invalid(path, code, raw string) FieldError {
	return FieldError{
		Path:    path,
		Code:    code,
		Message: fmt.Sprintf("field %s: invalid value %q", path, raw),
	}
}

func intCode(err error) string {
	if err == shape.ErrRange {
		return shape.CodeOutOfRange
	}
	return shape.CodeInvalidInt
}

func stringCode(err error) string {
	switch err {
	case shape.ErrTooLong:
		return shape.CodeTooLong
	case shape.ErrCharset:
		return shape.CodeInvalidCharset
	case shape.ErrEnum:
		return shape.CodeNotInEnum
	default:
		return shape.CodeInvalidValue
	}
}

func scalar(node any) (string, bool) {
	switch v := node.(type) {
	case string:
		return v, true
	case []any:
		if len(v) == 1 {
			if s, ok := v[0].(string); ok {
				return s, true
			}
		}
	}
	return "", false
}

func list(node any) ([]string, bool) {
	switch v := node.(type) {
	case string:
		return []string{v}, true
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	}
	return nil, false
}

func asExpandError(err error, target **shape.ExpandError) bool {
	ee, ok := err.(*shape.ExpandError)
	if !ok {
		return false
	}
	*target = ee
	return true
}
