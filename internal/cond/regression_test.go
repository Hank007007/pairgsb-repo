package cond

import "testing"

type typedMapEnv struct {
	values map[string]any
	types  map[string]string
}

func (e typedMapEnv) Get(path string) any { return e.values[path] }
func (e typedMapEnv) Type(path string) (string, bool) {
	typ, ok := e.types[path]
	return typ, ok
}
func (e typedMapEnv) Lookup(path string) (any, bool) {
	v, ok := e.values[path]
	return v, ok
}

func TestEvalNumericConversionUsesDeclaredType(t *testing.T) {
	for _, value := range []string{"10", "9"} {
		env := typedMapEnv{
			values: map[string]any{"principal.clearance": value},
			types:  map[string]string{"principal.clearance": "number"},
		}
		expr := `{"cmp":{"op":"gte","left":{"attr":"principal.clearance"},"right":3}}`
		if got := Eval(mustNode(t, expr), env); got != True {
			t.Errorf("clearance %q >= 3 = %v, want true", value, got)
		}
	}
}

func TestEvalBoolOrderingIsIndeterminate(t *testing.T) {
	env := typedMapEnv{
		values: map[string]any{"principal.mfa": true},
		types:  map[string]string{"principal.mfa": "bool"},
	}
	expr := `{"cmp":{"op":"gt","left":{"attr":"principal.mfa"},"right":false}}`
	if got := Eval(mustNode(t, expr), env); got != Indeterminate {
		t.Fatalf("bool gt = %v, want indeterminate", got)
	}
}

func TestEvalInScalarRequiresExactSegmentMatch(t *testing.T) {
	cases := []struct {
		name string
		env  typedMapEnv
		expr string
		want Result
	}{
		{
			name: "does not use substring matching",
			env: typedMapEnv{
				values: map[string]any{"principal.role": "admin"},
				types:  map[string]string{"principal.role": "string"},
			},
			expr: `{"in":{"path":"principal.role","values":["super-admin"]}}`,
			want: False,
		},
		{
			name: "single segment wildcard",
			env: typedMapEnv{
				values: map[string]any{"principal.tenant": "tenant:eu"},
				types:  map[string]string{"principal.tenant": "string"},
			},
			expr: `{"in":{"path":"principal.tenant","values":["tenant:*"]}}`,
			want: True,
		},
		{
			name: "wildcard does not span extra segments",
			env: typedMapEnv{
				values: map[string]any{"principal.tenant": "tenant:eu:finance"},
				types:  map[string]string{"principal.tenant": "string"},
			},
			expr: `{"in":{"path":"principal.tenant","values":["tenant:*"]}}`,
			want: False,
		},
		{
			name: "matching number of wildcards",
			env: typedMapEnv{
				values: map[string]any{"principal.tenant": "tenant:eu:finance"},
				types:  map[string]string{"principal.tenant": "string"},
			},
			expr: `{"in":{"path":"principal.tenant","values":["tenant:*:*"]}}`,
			want: True,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Eval(mustNode(t, tc.expr), tc.env); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEvalInArrayWildcard(t *testing.T) {
	env := typedMapEnv{
		values: map[string]any{"resource.tags": []any{"prod", "pii"}},
		types:  map[string]string{"resource.tags": "list"},
	}
	expr := `{"in":{"path":"resource.tags","values":["pro*"]}}`
	if got := Eval(mustNode(t, expr), env); got != False {
		t.Fatalf("partial-segment wildcard = %v, want false", got)
	}
	expr = `{"in":{"path":"resource.tags","values":["*"]}}`
	if got := Eval(mustNode(t, expr), env); got != True {
		t.Fatalf("single-segment wildcard = %v, want true", got)
	}
}

func TestEvalDeepPathOverArrayIsIndeterminate(t *testing.T) {
	env := typedMapEnv{
		values: map[string]any{"resource.tags": []any{"prod"}},
		types:  map[string]string{"resource.tags": "list"},
	}
	expr := `{"in":{"path":"resource.tags.env","values":["prod"]}}`
	if got := Eval(mustNode(t, expr), env); got != Indeterminate {
		t.Fatalf("resource.tags.env in = %v, want indeterminate", got)
	}
}

func TestEvalMissingReferencesAreIndeterminate(t *testing.T) {
	env := typedMapEnv{}
	exprs := []string{
		`{"not":{"cmp":{"op":"eq","left":{"attr":"principal.clearance"},"right":3}}}`,
		`{"not":{"contains":{"path":"resource.tags","value":"prod"}}}`,
	}
	for _, expr := range exprs {
		if got := Eval(mustNode(t, expr), env); got != Indeterminate {
			t.Errorf("%s = %v, want indeterminate", expr, got)
		}
	}
}

func TestEvalPresentMissingKeyIsFalse(t *testing.T) {
	env := typedMapEnv{}
	expr := `{"present":{"path":"principal.title"}}`
	if got := Eval(mustNode(t, expr), env); got != False {
		t.Fatalf("present missing = %v, want false", got)
	}
}

func TestEvalContainsString(t *testing.T) {
	env := typedMapEnv{
		values: map[string]any{"resource.owner": "team-atlas"},
		types:  map[string]string{"resource.owner": "string"},
	}
	expr := `{"contains":{"path":"resource.owner","value":"atlas"}}`
	if got := Eval(mustNode(t, expr), env); got != True {
		t.Fatalf("contains substring = %v, want true", got)
	}
}

func TestEvalUnknownNodeIsIndeterminate(t *testing.T) {
	node := mustNode(t, `{"unknown_operation": {}}`)
	if got := Eval(node, typedMapEnv{}); got != Indeterminate {
		t.Fatalf("unknown node = %v, want indeterminate", got)
	}
}
