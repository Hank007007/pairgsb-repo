package cond

import "testing"

// mapEnv is a simple environment backed by a nested map.
type mapEnv map[string]any

func (m mapEnv) Get(path string) any { return m[path] }
func (m mapEnv) Type(string) (string, bool) {
	return "", false
}

func mustNode(t *testing.T, raw string) *Node {
	t.Helper()
	var n Node
	if err := n.UnmarshalJSON([]byte(raw)); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return &n
}

func TestEvalCmpStrings(t *testing.T) {
	env := mapEnv{"principal.role": "analyst"}
	cases := []struct {
		expr string
		want Result
	}{
		{`{"cmp":{"op":"eq","left":{"attr":"principal.role"},"right":"analyst"}}`, True},
		{`{"cmp":{"op":"eq","left":{"attr":"principal.role"},"right":"admin"}}`, False},
		{`{"cmp":{"op":"ne","left":{"attr":"principal.role"},"right":"admin"}}`, True},
	}
	for _, tc := range cases {
		if got := Eval(mustNode(t, tc.expr), env); got != tc.want {
			t.Errorf("%s = %v, want %v", tc.expr, got, tc.want)
		}
	}
}

func TestEvalAndOrNot(t *testing.T) {
	env := mapEnv{
		"principal.role": "analyst",
		"principal.dept": "research",
	}
	expr := `{"and":[
		{"cmp":{"op":"eq","left":{"attr":"principal.role"},"right":"analyst"}},
		{"or":[
			{"cmp":{"op":"eq","left":{"attr":"principal.dept"},"right":"research"}},
			{"present":{"path":"principal.mfa"}}
		]}
	]}`
	if got := Eval(mustNode(t, expr), env); got != True {
		t.Fatalf("and/or = %v, want true", got)
	}
	negated := `{"not":{"cmp":{"op":"eq","left":{"attr":"principal.role"},"right":"analyst"}}}`
	if got := Eval(mustNode(t, negated), env); got != False {
		t.Fatalf("not = %v, want false", got)
	}
}

func TestEvalInList(t *testing.T) {
	env := mapEnv{"principal.groups": []any{"analyst", "oncall"}}
	expr := `{"in":{"path":"principal.groups","values":["oncall","admin"]}}`
	if got := Eval(mustNode(t, expr), env); got != True {
		t.Fatalf("in = %v, want true", got)
	}
	miss := `{"in":{"path":"principal.groups","values":["admin"]}}`
	if got := Eval(mustNode(t, miss), env); got != False {
		t.Fatalf("in miss = %v, want false", got)
	}
}

func TestEvalContainsList(t *testing.T) {
	env := mapEnv{"resource.tags": []any{"prod", "pii"}}
	expr := `{"contains":{"path":"resource.tags","value":"pii"}}`
	if got := Eval(mustNode(t, expr), env); got != True {
		t.Fatalf("contains = %v, want true", got)
	}
	expr = `{"contains":{"path":"resource.tags","value":"dev"}}`
	if got := Eval(mustNode(t, expr), env); got != False {
		t.Fatalf("contains miss = %v, want false", got)
	}
}

func TestEvalPresent(t *testing.T) {
	env := mapEnv{"principal.mfa": true}
	if got := Eval(mustNode(t, `{"present":{"path":"principal.mfa"}}`), env); got != True {
		t.Fatalf("present = %v, want true", got)
	}
	if got := Eval(mustNode(t, `{"present":{"path":"principal.clearance"}}`), env); got != False {
		t.Fatalf("present missing = %v, want false", got)
	}
}

func TestEvalEmptyCondition(t *testing.T) {
	if got := Eval(nil, mapEnv{}); got != True {
		t.Fatalf("nil condition = %v, want true", got)
	}
}
