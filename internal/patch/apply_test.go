package patch

import (
	"testing"
)

func mustOperations(t *testing.T, raw string) []Operation {
	t.Helper()
	ops, err := ParseRequest([]byte(raw))
	if err != nil {
		t.Fatalf("parse operations: %v", err)
	}
	return ops
}

func mustDocument(t *testing.T, raw string) any {
	t.Helper()
	doc, err := DecodeDocument([]byte(raw))
	if err != nil {
		t.Fatalf("decode document: %v", err)
	}
	return doc.Value
}

func field(t *testing.T, doc any, name string) any {
	t.Helper()
	obj, ok := doc.(map[string]any)
	if !ok {
		t.Fatalf("document is %T, want object", doc)
	}
	return obj[name]
}

func TestApplyReplaceAndAdd(t *testing.T) {
	doc := mustDocument(t, `{"title":"old","tags":["a","b"]}`)
	ops := mustOperations(t, `[
		{"op":"replace","path":"/title","value":"new"},
		{"op":"add","path":"/tags/-","value":"c"},
		{"op":"add","path":"/summary","value":"short"}
	]`)

	got, err := Apply(doc, ops, admin())
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if field(t, got, "title") != "new" {
		t.Fatalf("title = %v, want new", field(t, got, "title"))
	}
	tags, _ := field(t, got, "tags").([]any)
	if len(tags) != 3 || tags[2] != "c" {
		t.Fatalf("tags = %v", tags)
	}
	if field(t, got, "summary") != "short" {
		t.Fatalf("summary not added")
	}
}

func TestApplyRemoveAndMove(t *testing.T) {
	doc := mustDocument(t, `{"title":"t","metadata":{"k":"v"},"body":"b"}`)
	ops := mustOperations(t, `[
		{"op":"move","from":"/metadata/k","path":"/summary"},
		{"op":"remove","path":"/body"}
	]`)

	got, err := Apply(doc, ops, admin())
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, ok := got.(map[string]any)["body"]; ok {
		t.Fatalf("body not removed")
	}
	meta, _ := field(t, got, "metadata").(map[string]any)
	if len(meta) != 0 {
		t.Fatalf("metadata = %v, want empty", meta)
	}
}

func TestApplyTestSuccess(t *testing.T) {
	doc := mustDocument(t, `{"title":"stable","count":1}`)
	ops := mustOperations(t, `[
		{"op":"test","path":"/title","value":"stable"},
		{"op":"replace","path":"/title","value":"changed"}
	]`)

	got, err := Apply(doc, ops, admin())
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if field(t, got, "title") != "changed" {
		t.Fatalf("title = %v", field(t, got, "title"))
	}
}

func TestApplyMissingValue(t *testing.T) {
	doc := mustDocument(t, `{"title":"t"}`)
	ops := mustOperations(t, `[{"op":"replace","path":"/title"}]`)

	_, err := Apply(doc, ops, admin())
	if CodeOf(err) != CodeMissingMember {
		t.Fatalf("code = %q, want %q", CodeOf(err), CodeMissingMember)
	}
}
