package shape

import (
	"testing"
)

func TestExpandRejectsMalformedPath(t *testing.T) {
	_, err := Expand([]Entry{{Path: "a..b", Values: []string{"x"}}}, 3)
	if _, ok := err.(*ExpandError); !ok {
		t.Fatalf("err = %v, want *ExpandError", err)
	}
}

func TestLookupMissingReturnsFalse(t *testing.T) {
	tree, err := Expand([]Entry{{Path: "owner.email", Values: []string{"a@b.c"}}}, 3)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if _, ok := Lookup(tree, "owner.name"); ok {
		t.Fatal("Lookup found a missing field")
	}
}

func TestExpandEmptyValues(t *testing.T) {
	tree, err := Expand([]Entry{{Path: "bio", Values: nil}}, 3)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if tree["bio"] != "" {
		t.Fatalf("bio = %v", tree["bio"])
	}
}
