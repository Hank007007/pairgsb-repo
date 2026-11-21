package record

import (
	"sort"
	"strconv"
	"strings"
)

// FlatField is one leaf of a record's field tree.
type FlatField struct {
	Path  string
	Value any
}

// Flatten lists every leaf in the field tree with a canonical path. Map keys
// are visited in sorted order and array elements are indexed starting at 0,
// so the output depends only on the field contents.
func Flatten(fields map[string]any) []FlatField {
	var out []FlatField
	flattenInto(&out, "", fields)
	return out
}

func flattenInto(out *[]FlatField, prefix string, v any) {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			flattenInto(out, path, x[k])
		}
	case []any:
		for i, item := range x {
			path := prefix + "[" + strconv.Itoa(i) + "]"
			flattenInto(out, path, item)
		}
	case *Redacted:
		*out = append(*out, FlatField{Path: prefix, Value: x})
	default:
		*out = append(*out, FlatField{Path: prefix, Value: v})
	}
}

// SplitPath splits a canonical field path into segments. Array indices are
// returned as separate segments beginning with '['.
func SplitPath(path string) []string {
	var out []string
	var cur strings.Builder
	for _, r := range path {
		switch r {
		case '.':
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		case '[':
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			cur.WriteRune(r)
		case ']':
			cur.WriteRune(r)
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// CanonicalValue renders a single value exactly as it appears inside an
// encoded record.
func CanonicalValue(v any) (string, error) {
	var b strings.Builder
	if err := writeValue(&b, v); err != nil {
		return "", err
	}
	return b.String(), nil
}

// CloneFields returns a deep copy of fields so redaction never mutates the
// caller's map.
func CloneFields(fields map[string]any) map[string]any {
	if fields == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(fields))
	for k, v := range fields {
		out[k] = cloneValue(v)
	}
	return out
}

func cloneValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		return CloneFields(x)
	case []any:
		cp := make([]any, len(x))
		for i, item := range x {
			cp[i] = cloneValue(item)
		}
		return cp
	case Redacted:
		cp := x
		return &cp
	default:
		return v
	}
}
