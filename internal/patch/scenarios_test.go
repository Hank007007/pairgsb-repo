package patch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"patchsieve/internal/policy"
)

type scenarioFile struct {
	Scenarios []struct {
		Name      string           `json:"name"`
		Principal policy.Principal `json:"principal"`
		Document  json.RawMessage  `json:"document"`
		Patch     json.RawMessage  `json:"patch"`
	} `json:"scenarios"`
}

// scenarioErrors pins the expected error code ("" for success) of every
// scenario in testdata/scenarios.json.
var scenarioErrors = map[string]string{
	"escaped-slash-key":        "",
	"escaped-tilde-key":        "",
	"escaped-tilde-one-key":    "",
	"empty-key-nested":         "",
	"whole-document-pointer":   "",
	"invalid-escape":           CodeInvalidPointer,
	"trailing-tilde":           CodeInvalidPointer,
	"array-add-out-of-range":   CodeIndexOutOfRange,
	"array-add-insert":         "",
	"array-add-append":         "",
	"array-negative-index":     CodeInvalidIndex,
	"array-index-out-of-range": CodeIndexOutOfRange,
	"missing-member-target":    CodePathNotFound,
	"scalar-midpath":           CodePathNotContainer,
	"null-midpath":             CodePathNotContainer,
	"move-into-descendant":     CodeInvalidMove,
	"move-onto-itself":         "",
	"unknown-op":               CodeUnknownOp,
	"read-only-role":           CodeFieldReadOnly,
	"read-only-acl":            CodeFieldReadOnly,
	"read-only-tenant":         CodeFieldReadOnly,
	"move-from-read-only":      CodeFieldReadOnly,
	"viewer-denied":            CodeFieldReadOnly,
	"editor-status-denied":     CodeFieldReadOnly,
	"atomic-rollback":          CodeTestFailed,
	"numeric-equality":         "",
	"deep-copy":                "",
	"limit-depth":              CodeLimitExceeded,
	"limit-operations":         CodeLimitExceeded,
	"limit-string-length":      CodeLimitExceeded,
}

func TestScenarios(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "scenarios.json"))
	if err != nil {
		t.Fatalf("read scenarios: %v", err)
	}
	var file scenarioFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("parse scenarios: %v", err)
	}

	seen := make(map[string]bool)
	for _, sc := range file.Scenarios {
		t.Run(sc.Name, func(t *testing.T) {
			seen[sc.Name] = true
			wantCode, ok := scenarioErrors[sc.Name]
			if !ok {
				t.Fatalf("no expectation recorded for scenario %q", sc.Name)
			}

			doc, err := DecodeDocument(sc.Document)
			if err != nil {
				t.Fatalf("decode document: %v", err)
			}
			original, err := DecodeDocument(sc.Document)
			if err != nil {
				t.Fatalf("decode document: %v", err)
			}
			ops, err := ParseRequest(sc.Patch)
			if err != nil {
				t.Fatalf("parse patch: %v", err)
			}

			result, applyErr := Apply(doc.Value, ops, sc.Principal)
			if code := CodeOf(applyErr); code != wantCode {
				t.Fatalf("error code = %q, want %q (err: %v)", code, wantCode, applyErr)
			}

			// The input document is never modified.
			if !reflect.DeepEqual(doc.Value, original.Value) {
				t.Fatalf("input document mutated: got %v, want %v", doc.Value, original.Value)
			}
			// On failure the original document is returned unchanged.
			if applyErr != nil && !reflect.DeepEqual(result, original.Value) {
				t.Fatalf("failed apply returned %v, want unchanged %v", result, original.Value)
			}
		})
	}
	for name := range scenarioErrors {
		if !seen[name] {
			t.Errorf("expectation for %q has no matching scenario", name)
		}
	}
}

func TestScenarioDeepCopyIndependence(t *testing.T) {
	doc := mustDocument(t, `{"metadata":{"tags":["a"]},"title":"t"}`)
	ops := mustOperations(t, `[
		{"op":"copy","from":"/metadata","path":"/summary"},
		{"op":"replace","path":"/summary/tags/0","value":"b"}
	]`)

	got, err := Apply(doc, ops, editor())
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	source := field(t, got, "metadata").(map[string]any)["tags"].([]any)
	if source[0] != "a" {
		t.Fatalf("source tags[0] = %v, want a (copy must be independent)", source[0])
	}
	copied := field(t, got, "summary").(map[string]any)["tags"].([]any)
	if copied[0] != "b" {
		t.Fatalf("copied tags[0] = %v, want b", copied[0])
	}
}

func TestScenarioAtomicRollback(t *testing.T) {
	doc := mustDocument(t, `{"title":"t","count":1}`)
	ops := mustOperations(t, `[
		{"op":"replace","path":"/title","value":"changed"},
		{"op":"test","path":"/count","value":2}
	]`)

	got, err := Apply(doc, ops, editor())
	if CodeOf(err) != CodeTestFailed {
		t.Fatalf("code = %q, want %q", CodeOf(err), CodeTestFailed)
	}
	if field(t, got, "title") != "t" {
		t.Fatalf("title = %v, want t (no partial application)", field(t, got, "title"))
	}
}
