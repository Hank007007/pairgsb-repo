package main

import (
	"encoding/json"
	"strings"
	"testing"

	"strictcast/internal/shape"
)

func TestFlattenJSONBuildsPaths(t *testing.T) {
	var doc any
	dec := json.NewDecoder(strings.NewReader(`{"username":"ada","owner":{"email":"a@b.c"},"tags":["x","y"],"attempts":3}`))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var entries []shape.Entry
	flattenJSON("", doc, &entries)

	got := map[string][]string{}
	for _, e := range entries {
		got[e.Path] = e.Values
	}
	want := map[string][]string{
		"username":    {"ada"},
		"owner.email": {"a@b.c"},
		"tags[0]":     {"x"},
		"tags[1]":     {"y"},
		"attempts":    {"3"},
	}
	for path, values := range want {
		if !equalStrings(got[path], values) {
			t.Fatalf("%s = %v, want %v", path, got[path], values)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("entries = %v", got)
	}
}

func TestFormEntriesOnePerKey(t *testing.T) {
	entries := formEntries(map[string][]string{
		"tags[]":   {"a", "b"},
		"username": {"ada"},
	})
	if len(entries) != 2 {
		t.Fatalf("entries = %v", entries)
	}
	if entries[0].Path != "tags[]" || !equalStrings(entries[0].Values, []string{"a", "b"}) {
		t.Fatalf("first entry = %+v", entries[0])
	}
	if entries[1].Path != "username" {
		t.Fatalf("second entry = %+v", entries[1])
	}
}

func TestIsJSON(t *testing.T) {
	cases := map[string]bool{
		"application/json":                  true,
		"Application/JSON; charset=utf-8":   true,
		"application/x-www-form-urlencoded": false,
		"":                                  false,
	}
	for ct, want := range cases {
		if got := isJSON(ct); got != want {
			t.Fatalf("isJSON(%q) = %v, want %v", ct, got, want)
		}
	}
}

func TestTrimSlash(t *testing.T) {
	cases := map[string]string{
		"/profile.update": "profile.update",
		"profile.update/": "profile.update",
		"//a/b//":         "a/b",
		"":                "",
	}
	for in, want := range cases {
		if got := trimSlash(in); got != want {
			t.Fatalf("trimSlash(%q) = %q, want %q", in, got, want)
		}
	}
}

func equalStrings(a, b []string) bool {
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
