package gate

import (
	"reflect"
	"testing"

	"strictcast/internal/shape"
)

func accountShape() *shape.Shape {
	return shape.Registry["account.create"]
}

func TestBindHappyPathJSONStyle(t *testing.T) {
	entries := []shape.Entry{
		{Path: "username", Values: []string{"ada"}},
		{Path: "display_name", Values: []string{"Ada Lovelace"}},
		{Path: "attempts", Values: []string{"3"}},
		{Path: "enabled", Values: []string{"true"}},
		{Path: "owner.email", Values: []string{"ada@example.com"}},
		{Path: "items[0]", Values: []string{"go"}},
		{Path: "items[1]", Values: []string{"api"}},
	}
	res, bindErr := Bind(accountShape(), entries)
	if bindErr != nil {
		t.Fatalf("Bind: %v", bindErr)
	}

	if v, ok := res.String("username"); !ok || v != "ada" {
		t.Fatalf("username = %q, %v", v, ok)
	}
	if v, ok := res.String("display_name"); !ok || v != "Ada Lovelace" {
		t.Fatalf("display_name = %q, %v", v, ok)
	}
	if v, ok := res.Int("attempts"); !ok || v != 3 {
		t.Fatalf("attempts = %d, %v", v, ok)
	}
	if v, ok := res.Bool("enabled"); !ok || v != true {
		t.Fatalf("enabled = %v, %v", v, ok)
	}
	if v, ok := res.String("owner.email"); !ok || v != "ada@example.com" {
		t.Fatalf("owner.email = %q, %v", v, ok)
	}
	if v, ok := res.Strings("items"); !ok || !reflect.DeepEqual(v, []string{"go", "api"}) {
		t.Fatalf("items = %v, %v", v, ok)
	}
}

func TestBindFormStyleList(t *testing.T) {
	entries := []shape.Entry{
		{Path: "username", Values: []string{"ada"}},
		{Path: "display_name", Values: []string{"Ada"}},
		{Path: "attempts", Values: []string{"0"}},
		{Path: "enabled", Values: []string{"false"}},
		{Path: "owner.email", Values: []string{"ada@example.com"}},
		{Path: "items[]", Values: []string{"x", "y", "z"}},
	}
	res, bindErr := Bind(accountShape(), entries)
	if bindErr != nil {
		t.Fatalf("Bind: %v", bindErr)
	}
	if v, ok := res.Strings("items"); !ok || !reflect.DeepEqual(v, []string{"x", "y", "z"}) {
		t.Fatalf("items = %v, %v", v, ok)
	}
}

func TestBindReportsAllMissing(t *testing.T) {
	_, bindErr := Bind(accountShape(), nil)
	if bindErr == nil {
		t.Fatal("Bind accepted an empty request")
	}
	got := map[string]string{}
	for _, fe := range bindErr.Errors {
		got[fe.Path] = fe.Code
	}
	want := map[string]string{
		"username":     shape.CodeMissing,
		"display_name": shape.CodeMissing,
		"attempts":     shape.CodeMissing,
		"enabled":      shape.CodeMissing,
		"owner.email":  shape.CodeMissing,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("errors = %v, want %v", got, want)
	}
}

func TestBindErrorsSortedByPath(t *testing.T) {
	_, bindErr := Bind(accountShape(), nil)
	if bindErr == nil {
		t.Fatal("expected errors")
	}
	for i := 1; i < len(bindErr.Errors); i++ {
		if bindErr.Errors[i-1].Path > bindErr.Errors[i].Path {
			t.Fatalf("errors not sorted: %v", bindErr.Errors)
		}
	}
}

func TestBindBadPath(t *testing.T) {
	sh := &shape.Shape{Name: "t", MaxDepth: 3, Fields: map[string]shape.Field{}}
	_, bindErr := Bind(sh, []shape.Entry{{Path: "a..b", Values: []string{"x"}}})
	if bindErr == nil {
		t.Fatal("expected bad_path")
	}
	if bindErr.Errors[0].Code != shape.CodeBadPath {
		t.Fatalf("code = %q", bindErr.Errors[0].Code)
	}
}

func TestBindSingleStringField(t *testing.T) {
	sh, ok := shape.Get("profile.update")
	if !ok {
		t.Fatal("profile.update not registered")
	}
	entries := []shape.Entry{
		{Path: "nickname", Values: []string{"ok"}},
	}
	res, bindErr := Bind(sh, entries)
	if bindErr != nil {
		t.Fatalf("Bind: %v", bindErr)
	}
	if v, ok := res.String("nickname"); !ok || v != "ok" {
		t.Fatalf("nickname = %q, %v", v, ok)
	}
}

func TestBindUnknownKeysIgnored(t *testing.T) {
	sh, ok := shape.Get("profile.update")
	if !ok {
		t.Fatal("profile.update not registered")
	}
	res, bindErr := Bind(sh, []shape.Entry{
		{Path: "nickname", Values: []string{"ok"}},
		{Path: "not_registered", Values: []string{"whatever"}},
	})
	if bindErr != nil {
		t.Fatalf("Bind: %v", bindErr)
	}
	if _, ok := res.Value("not_registered"); ok {
		t.Fatal("unknown key was bound")
	}
}

func TestBindNilShape(t *testing.T) {
	_, bindErr := Bind(nil, nil)
	if bindErr == nil {
		t.Fatal("expected an error for a nil shape")
	}
	if bindErr.Errors[0].Code != shape.CodeBadPath {
		t.Fatalf("code = %q", bindErr.Errors[0].Code)
	}
}

func TestIntErrorCodes(t *testing.T) {
	cases := []struct {
		err  error
		code string
	}{
		{shape.ErrRange, shape.CodeOutOfRange},
		{shape.ErrInt, shape.CodeInvalidInt},
	}
	for _, tc := range cases {
		if got := intCode(tc.err); got != tc.code {
			t.Fatalf("intCode(%v) = %q, want %q", tc.err, got, tc.code)
		}
	}
}

func TestStringErrorCodes(t *testing.T) {
	cases := []struct {
		err  error
		code string
	}{
		{shape.ErrTooLong, shape.CodeTooLong},
		{shape.ErrCharset, shape.CodeInvalidCharset},
		{shape.ErrEnum, shape.CodeNotInEnum},
		{shape.ErrInt, shape.CodeInvalidValue},
	}
	for _, tc := range cases {
		if got := stringCode(tc.err); got != tc.code {
			t.Fatalf("stringCode(%v) = %q, want %q", tc.err, got, tc.code)
		}
	}
}
