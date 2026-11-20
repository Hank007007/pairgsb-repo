package cond

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

// Result is the three-valued outcome of evaluating a condition.
type Result int

const (
	// False means the expression is decisively not satisfied.
	False Result = iota
	// True means the expression is decisively satisfied.
	True
	// Indeterminate means the expression could not be evaluated.
	Indeterminate
)

// String returns the name of the result.
func (r Result) String() string {
	switch r {
	case True:
		return "true"
	case False:
		return "false"
	default:
		return "indeterminate"
	}
}

// Env supplies attribute values to the evaluator. Get returns nil when the
// path does not exist. Type reports the declared type of an attribute path.
type Env interface {
	Get(path string) any
	Type(path string) (string, bool)
}

// lookupEnv is implemented by environments that can distinguish a missing
// attribute from an attribute whose value is null.
type lookupEnv interface {
	Lookup(path string) (any, bool)
}

func lookup(env Env, path string) (any, bool) {
	if e, ok := env.(lookupEnv); ok {
		return e.Lookup(path)
	}
	v := env.Get(path)
	return v, v != nil
}

// Eval evaluates a condition tree against env.
func Eval(n *Node, env Env) Result {
	if n == nil {
		return True
	}
	if n.Invalid {
		return Indeterminate
	}
	switch {
	case n.And != nil:
		return evalAnd(n.And, env)
	case n.Or != nil:
		return evalOr(n.Or, env)
	case n.Not != nil:
		return evalNot(*n.Not, env)
	case n.Cmp != nil:
		return evalCmp(*n.Cmp, env)
	case n.In != nil:
		return evalIn(*n.In, env)
	case n.Contains != nil:
		return evalContains(*n.Contains, env)
	case n.Present != nil:
		return evalPresent(*n.Present, env)
	}
	return Indeterminate
}

func evalAnd(nodes []Node, env Env) Result {
	result := True
	for i := range nodes {
		switch Eval(&nodes[i], env) {
		case False:
			return False
		case Indeterminate:
			result = Indeterminate
		}
	}
	return result
}

func evalOr(nodes []Node, env Env) Result {
	result := False
	for i := range nodes {
		switch Eval(&nodes[i], env) {
		case True:
			return True
		case Indeterminate:
			result = Indeterminate
		}
	}
	return result
}

func evalNot(n Node, env Env) Result {
	switch Eval(&n, env) {
	case True:
		return False
	case False:
		return True
	default:
		return Indeterminate
	}
}

type refValue struct {
	attr    bool
	present bool
	raw     any
	typ     string
}

func resolveRef(ref Ref, env Env) refValue {
	if ref.IsAttr {
		v, ok := lookup(env, ref.Attr)
		if !ok {
			return refValue{attr: true}
		}
		typ := ""
		if t, ok := env.Type(ref.Attr); ok {
			typ = normalizeType(t)
		}
		if typ == "" {
			typ = inferType(v)
		}
		return refValue{attr: true, present: true, raw: v, typ: typ}
	}
	return refValue{present: true, raw: ref.Literal, typ: inferType(ref.Literal)}
}

func evalCmp(c Cmp, env Env) Result {
	op := strings.ToLower(strings.TrimSpace(c.Op))
	left := resolveRef(c.Left, env)
	right := resolveRef(c.Right, env)
	if !left.present || !right.present {
		return Indeterminate
	}

	switch {
	case c.Left.IsAttr && c.Right.IsAttr:
		if left.typ == "" || right.typ == "" || left.typ != right.typ {
			return Indeterminate
		}
		lv, lok := convert(left.raw, left.typ)
		rv, rok := convert(right.raw, right.typ)
		if !lok || !rok {
			return Indeterminate
		}
		return compareTyped(lv, rv, op)
	case c.Left.IsAttr:
		if left.typ == "" {
			return Indeterminate
		}
		lv, lok := convert(left.raw, left.typ)
		rv, rok := convert(right.raw, left.typ)
		if !lok || !rok {
			return Indeterminate
		}
		return compareTyped(lv, rv, op)
	case c.Right.IsAttr:
		if right.typ == "" {
			return Indeterminate
		}
		lv, lok := convert(left.raw, right.typ)
		rv, rok := convert(right.raw, right.typ)
		if !lok || !rok {
			return Indeterminate
		}
		return compareTyped(lv, rv, op)
	default:
		return compareRaw(left.raw, right.raw, op)
	}
}

type typedValue struct {
	kind string
	s    string
	n    float64
	b    bool
	list []string
}

func convert(v any, typ string) (typedValue, bool) {
	switch normalizeType(typ) {
	case "string":
		s, ok := v.(string)
		if !ok {
			return typedValue{}, false
		}
		return typedValue{kind: "string", s: s}, true
	case "number":
		n, ok := toNumber(v)
		if !ok {
			return typedValue{}, false
		}
		return typedValue{kind: "number", n: n}, true
	case "bool":
		b, ok := toBool(v)
		if !ok {
			return typedValue{}, false
		}
		return typedValue{kind: "bool", b: b}, true
	case "list":
		list, ok := toStringList(v)
		if !ok {
			return typedValue{}, false
		}
		return typedValue{kind: "list", list: list}, true
	default:
		return typedValue{}, false
	}
}

func compareTyped(l, r typedValue, op string) Result {
	if l.kind != r.kind {
		return Indeterminate
	}
	switch l.kind {
	case "string":
		switch op {
		case "eq":
			return fromBool(l.s == r.s)
		case "ne":
			return fromBool(l.s != r.s)
		case "lt":
			return fromBool(l.s < r.s)
		case "lte":
			return fromBool(l.s <= r.s)
		case "gt":
			return fromBool(l.s > r.s)
		case "gte":
			return fromBool(l.s >= r.s)
		}
	case "number":
		switch op {
		case "eq":
			return fromBool(l.n == r.n)
		case "ne":
			return fromBool(l.n != r.n)
		case "lt":
			return fromBool(l.n < r.n)
		case "lte":
			return fromBool(l.n <= r.n)
		case "gt":
			return fromBool(l.n > r.n)
		case "gte":
			return fromBool(l.n >= r.n)
		}
	case "bool":
		switch op {
		case "eq":
			return fromBool(l.b == r.b)
		case "ne":
			return fromBool(l.b != r.b)
		}
	case "list":
		switch op {
		case "eq":
			return fromBool(stringSlicesEqual(l.list, r.list))
		case "ne":
			return fromBool(!stringSlicesEqual(l.list, r.list))
		}
	}
	return Indeterminate
}

func compareRaw(l, r any, op string) Result {
	switch op {
	case "eq":
		return fromBool(reflect.DeepEqual(l, r))
	case "ne":
		return fromBool(!reflect.DeepEqual(l, r))
	}

	lk, rk := jsonKind(l), jsonKind(r)
	if lk == "" || lk != rk {
		return Indeterminate
	}
	switch lk {
	case "number":
		ln, lok := toNumber(l)
		rn, rok := toNumber(r)
		if !lok || !rok {
			return Indeterminate
		}
		return compareTyped(typedValue{kind: "number", n: ln}, typedValue{kind: "number", n: rn}, op)
	case "string":
		ls, lok := l.(string)
		rs, rok := r.(string)
		if !lok || !rok {
			return Indeterminate
		}
		return compareTyped(typedValue{kind: "string", s: ls}, typedValue{kind: "string", s: rs}, op)
	}
	return Indeterminate
}

func evalIn(in In, env Env) Result {
	actual, ok := lookup(env, in.Path)
	if !ok {
		return Indeterminate
	}
	wanted := make([]refValue, 0, len(in.Values))
	for _, ref := range in.Values {
		wanted = append(wanted, resolveRef(ref, env))
	}

	switch value := actual.(type) {
	case []any:
		return matchElements(value, wanted, true)
	case []string:
		items := make([]any, len(value))
		for i, item := range value {
			items[i] = item
		}
		return matchElements(items, wanted, true)
	case string:
		return matchScalar(value, wanted)
	default:
		return Indeterminate
	}
}

func matchScalar(actual string, wanted []refValue) Result {
	uncertain := false
	for _, want := range wanted {
		if !want.present {
			uncertain = true
			continue
		}
		value, ok := convert(want.raw, "string")
		if !ok {
			uncertain = true
			continue
		}
		if matchPattern(value.s, actual) {
			return True
		}
	}
	if uncertain {
		return Indeterminate
	}
	return False
}

func matchElements(items []any, wanted []refValue, wildcard bool) Result {
	uncertain := false
	for _, item := range items {
		typ := inferType(item)
		if typ == "" {
			uncertain = true
			continue
		}
		actual, ok := convert(item, typ)
		if !ok {
			uncertain = true
			continue
		}
		for _, want := range wanted {
			if !want.present {
				uncertain = true
				continue
			}
			candidate, ok := convert(want.raw, typ)
			if !ok {
				uncertain = true
				continue
			}
			var result Result
			if wildcard && typ == "string" {
				result = fromBool(matchPattern(candidate.s, actual.s))
			} else {
				result = compareTyped(actual, candidate, "eq")
			}
			switch result {
			case True:
				return True
			case Indeterminate:
				uncertain = true
			}
		}
	}
	if uncertain {
		return Indeterminate
	}
	return False
}

func evalContains(c Contains, env Env) Result {
	actual, ok := lookup(env, c.Path)
	if !ok {
		return Indeterminate
	}
	want := resolveRef(c.Value, env)
	if !want.present {
		return Indeterminate
	}

	switch value := actual.(type) {
	case []any:
		return matchElements(value, []refValue{want}, false)
	case []string:
		items := make([]any, len(value))
		for i, item := range value {
			items[i] = item
		}
		return matchElements(items, []refValue{want}, false)
	case string:
		s, ok := convert(want.raw, "string")
		if !ok {
			return Indeterminate
		}
		return fromBool(strings.Contains(value, s.s))
	default:
		return Indeterminate
	}
}

func evalPresent(p Present, env Env) Result {
	v, ok := lookup(env, p.Path)
	return fromBool(ok && v != nil)
}

func matchPattern(pattern, value string) bool {
	patternParts := strings.Split(pattern, ":")
	valueParts := strings.Split(value, ":")
	if len(patternParts) != len(valueParts) {
		return false
	}
	for i := range patternParts {
		if patternParts[i] != "*" && patternParts[i] != valueParts[i] {
			return false
		}
	}
	return true
}

func normalizeType(t string) string {
	return strings.ToLower(strings.TrimSpace(t))
}

func inferType(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case bool:
		return "bool"
	case float32, float64, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, json.Number:
		return "number"
	case []any, []string:
		return "list"
	}
	return ""
}

func jsonKind(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "bool"
	case float32, float64, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, json.Number:
		return "number"
	case []any, []string:
		return "list"
	case map[string]any:
		return "object"
	default:
		return ""
	}
}

func toNumber(v any) (float64, bool) {
	var n float64
	switch value := v.(type) {
	case json.Number:
		parsed, err := strconv.ParseFloat(value.String(), 64)
		if err != nil {
			return 0, false
		}
		n = parsed
	case float64:
		n = value
	case float32:
		n = float64(value)
	case int:
		n = float64(value)
	case int8:
		n = float64(value)
	case int16:
		n = float64(value)
	case int32:
		n = float64(value)
	case int64:
		n = float64(value)
	case uint:
		n = float64(value)
	case uint8:
		n = float64(value)
	case uint16:
		n = float64(value)
	case uint32:
		n = float64(value)
	case uint64:
		n = float64(value)
	case string:
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, false
		}
		n = parsed
	default:
		return 0, false
	}
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, false
	}
	return n, true
}

func toBool(v any) (bool, bool) {
	switch value := v.(type) {
	case bool:
		return value, true
	case string:
		switch value {
		case "true":
			return true, true
		case "false":
			return false, true
		}
	}
	return false, false
}

func toStringList(v any) ([]string, bool) {
	switch value := v.(type) {
	case []string:
		out := make([]string, len(value))
		copy(out, value)
		return out, true
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			switch item := item.(type) {
			case string:
				out = append(out, item)
			default:
				out = append(out, fmt.Sprint(item))
			}
		}
		return out, true
	}
	return nil, false
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func fromBool(v bool) Result {
	if v {
		return True
	}
	return False
}
