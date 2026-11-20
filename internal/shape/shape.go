// Package shape describes request shapes and the primitives used to turn
// submitted text into typed values.
package shape

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Kind identifies the target Go type of a field.
type Kind string

const (
	KindString     Kind = "string"
	KindInt        Kind = "int"
	KindBool       Kind = "bool"
	KindStringList Kind = "string_list"
)

// Class identifies the character repertoire allowed for a text field.
type Class string

const (
	// ClassAny applies no character restriction.
	ClassAny Class = "any"
	// ClassText allows printable Unicode text (including CJK and emoji).
	ClassText Class = "text"
	// ClassSafe allows the restricted ASCII repertoire used for codes,
	// identifiers and short slugs.
	ClassSafe Class = "safe"
)

// Field describes one registered field.
//
// MaxBytes is the upper bound on the UTF-8 encoded length of the value (for
// list fields it applies to each element). A non-positive MaxBytes means the
// field is unbounded. Enum, when non-empty, lists the accepted values.
type Field struct {
	Kind     Kind
	Required bool
	MaxBytes int
	Class    Class
	Enum     []string
}

// Shape is the set of fields registered for one endpoint.
//
// MaxDepth is the maximum number of path segments (name segments and index
// segments together) accepted in a single submitted key.
type Shape struct {
	Name     string
	MaxDepth int
	Fields   map[string]Field
}

// FieldNames returns the registered field paths in a deterministic order.
func (s *Shape) FieldNames() []string {
	names := make([]string, 0, len(s.Fields))
	for name := range s.Fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Sentinel errors reported by the coercion primitives.
var (
	ErrInt     = errors.New("shape: not an integer")
	ErrRange   = errors.New("shape: integer outside target width")
	ErrBool    = errors.New("shape: not a boolean literal")
	ErrCharset = errors.New("shape: character outside field class")
	ErrTooLong = errors.New("shape: value exceeds byte limit")
	ErrEnum    = errors.New("shape: value not in enum")
)

// Error codes reported for a single field.
const (
	CodeMissing        = "missing"
	CodeInvalidInt     = "invalid_int"
	CodeOutOfRange     = "out_of_range"
	CodeInvalidBool    = "invalid_bool"
	CodeInvalidCharset = "invalid_charset"
	CodeTooLong        = "too_long"
	CodeNotInEnum      = "not_in_enum"
	CodeInvalidValue   = "invalid_value"
)

// Structural error codes reported while resolving submitted keys.
const (
	CodeBadPath   = "bad_path"
	CodeDuplicate = "duplicate"
	CodeConflict  = "path_conflict"
	CodeTooDeep   = "too_deep"
)

// ExpandError reports a key that cannot be resolved into the request tree.
type ExpandError struct {
	Path string
	Code string
}

func (e *ExpandError) Error() string {
	return fmt.Sprintf("shape: %s at %s", e.Code, e.Path)
}

// Entry is one submitted key with its values in submission order.
//
// Keys from a form and member names from a JSON body are both written in the
// path syntax described in docs/shape-contract.md and resolved the same way.
type Entry struct {
	Path   string
	Values []string
}

type segment struct {
	Name     string
	Index    int
	IsIndex  bool
	Wildcard bool
}

func parsePath(path string) ([]segment, error) {
	if path == "" {
		return nil, &ExpandError{Path: path, Code: CodeBadPath}
	}
	var segs []segment
	i := 0
	expectName := true
	for i < len(path) {
		if expectName {
			start := i
			for i < len(path) && path[i] != '.' && path[i] != '[' {
				i++
			}
			if i == start {
				return nil, &ExpandError{Path: path, Code: CodeBadPath}
			}
			name := path[start:i]
			if strings.ContainsAny(name, "[]") {
				return nil, &ExpandError{Path: path, Code: CodeBadPath}
			}
			segs = append(segs, segment{Name: name})
			expectName = false
			continue
		}
		switch path[i] {
		case '.':
			i++
			if i >= len(path) {
				return nil, &ExpandError{Path: path, Code: CodeBadPath}
			}
			expectName = true
		case '[':
			end := strings.IndexByte(path[i:], ']')
			if end < 0 {
				return nil, &ExpandError{Path: path, Code: CodeBadPath}
			}
			inner := path[i+1 : i+end]
			if inner == "" {
				segs = append(segs, segment{IsIndex: true, Wildcard: true})
			} else {
				n, err := strconv.Atoi(inner)
				if err != nil || n < 0 {
					return nil, &ExpandError{Path: path, Code: CodeBadPath}
				}
				segs = append(segs, segment{IsIndex: true, Index: n})
			}
			i += end + 1
		default:
			return nil, &ExpandError{Path: path, Code: CodeBadPath}
		}
	}
	if expectName {
		return nil, &ExpandError{Path: path, Code: CodeBadPath}
	}
	return segs, nil
}

func lastValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

func assignNode(container any, segs []segment, values []string) (any, error) {
	if len(segs) == 0 {
		return lastValue(values), nil
	}
	seg := segs[0]
	if !seg.IsIndex {
		m, _ := container.(map[string]any)
		if m == nil {
			m = map[string]any{}
		}
		child, err := assignNode(m[seg.Name], segs[1:], values)
		if err != nil {
			return nil, err
		}
		m[seg.Name] = child
		return m, nil
	}
	list, _ := container.([]any)
	if seg.Wildcard {
		if len(segs) == 1 {
			if len(values) == 0 {
				list = append(list, "")
			}
			for _, v := range values {
				list = append(list, v)
			}
		} else {
			child, err := assignNode(nil, segs[1:], values)
			if err != nil {
				return nil, err
			}
			list = append(list, child)
		}
		return list, nil
	}
	for len(list) <= seg.Index {
		list = append(list, nil)
	}
	child, err := assignNode(list[seg.Index], segs[1:], values)
	if err != nil {
		return nil, err
	}
	list[seg.Index] = child
	return list, nil
}

// Expand resolves submitted keys into a nested tree of maps, lists and string
// leaves. Segments are resolved in submission order.
func Expand(entries []Entry, maxDepth int) (map[string]any, error) {
	root := map[string]any{}
	for _, entry := range entries {
		segs, err := parsePath(entry.Path)
		if err != nil {
			return nil, err
		}
		node, err := assignNode(root, segs, entry.Values)
		if err != nil {
			return nil, err
		}
		root, _ = node.(map[string]any)
	}
	return root, nil
}

// Lookup resolves a registered field path against an expanded tree.
func Lookup(root map[string]any, path string) (any, bool) {
	segs, err := parsePath(path)
	if err != nil {
		return nil, false
	}
	var cur any = root
	for _, seg := range segs {
		if seg.IsIndex {
			list, ok := cur.([]any)
			if !ok || seg.Wildcard || seg.Index >= len(list) {
				return nil, false
			}
			cur = list[seg.Index]
			continue
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := m[seg.Name]
		if !ok {
			return nil, false
		}
		cur = v
	}
	return cur, true
}
