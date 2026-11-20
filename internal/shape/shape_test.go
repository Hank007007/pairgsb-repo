package shape

import (
	"reflect"
	"testing"
)

func entry(path string, values ...string) Entry {
	return Entry{Path: path, Values: values}
}

func TestExpandNestedPaths(t *testing.T) {
	entries := []Entry{
		entry("owner.email", "ada@example.com"),
		entry("owner.name", "Ada"),
		entry("tags[0]", "go"),
		entry("tags[1]", "api"),
	}
	tree, err := Expand(entries, 3)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	email, ok := Lookup(tree, "owner.email")
	if !ok || email != "ada@example.com" {
		t.Fatalf("owner.email = %v, %v", email, ok)
	}
	name, ok := Lookup(tree, "owner.name")
	if !ok || name != "Ada" {
		t.Fatalf("owner.name = %v, %v", name, ok)
	}
	tags, ok := Lookup(tree, "tags[1]")
	if !ok || tags != "api" {
		t.Fatalf("tags[1] = %v, %v", tags, ok)
	}

	owner, ok := tree["owner"].(map[string]any)
	if !ok {
		t.Fatalf("owner is %T", tree["owner"])
	}
	if len(owner) != 2 {
		t.Fatalf("owner has %d members", len(owner))
	}
}

func TestExpandWildcardAppends(t *testing.T) {
	tree, err := Expand([]Entry{entry("tags[]", "a", "b", "c")}, 3)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	got, ok := tree["tags"].([]any)
	if !ok {
		t.Fatalf("tags is %T", tree["tags"])
	}
	want := []any{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tags = %v, want %v", got, want)
	}
}

func TestExpandSingleValueIsLeaf(t *testing.T) {
	tree, err := Expand([]Entry{entry("username", "ada")}, 3)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if tree["username"] != "ada" {
		t.Fatalf("username = %v", tree["username"])
	}
}

func TestParseRejectsMalformedPaths(t *testing.T) {
	cases := []string{"", "a..b", ".a", "a.", "a[", "a[x]", "a[-1]", "a]b"}
	for _, path := range cases {
		_, err := Expand([]Entry{entry(path, "v")}, 3)
		ee, ok := err.(*ExpandError)
		if !ok {
			t.Fatalf("path %q: err = %v, want *ExpandError", path, err)
		}
		if ee.Code != CodeBadPath {
			t.Fatalf("path %q: code = %q", path, ee.Code)
		}
	}
}

func TestRegistryFieldNamesSorted(t *testing.T) {
	sh, ok := Get("profile.update")
	if !ok {
		t.Fatal("profile.update not registered")
	}
	got := sh.FieldNames()
	want := []string{"code", "nickname", "note"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FieldNames = %v, want %v", got, want)
	}
}
