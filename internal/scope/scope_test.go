package scope

import "testing"

func mustParse(t *testing.T, raw string) Scope {
	t.Helper()
	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse(%q): %v", raw, err)
	}
	return s
}

func TestParseSplitsOnLastColon(t *testing.T) {
	s := mustParse(t, "s3:read:bucket/incident-42")
	if s.Action != "s3:read" {
		t.Fatalf("action = %q, want %q", s.Action, "s3:read")
	}
	if s.Resource != "bucket/incident-42" {
		t.Fatalf("resource = %q, want %q", s.Resource, "bucket/incident-42")
	}
	if got := s.String(); got != "s3:read:bucket/incident-42" {
		t.Fatalf("String() = %q", got)
	}
}

func TestParseRejectsMalformed(t *testing.T) {
	for _, raw := range []string{"", "   ", "s3read", ":bucket/a", "s3:read:"} {
		if _, err := Parse(raw); err == nil {
			t.Fatalf("Parse(%q) succeeded, want error", raw)
		}
	}
}

func TestParseAllRejectsDuplicates(t *testing.T) {
	if _, err := ParseAll([]string{"s3:read:bucket/a", "s3:read:bucket/a"}); err == nil {
		t.Fatal("ParseAll accepted duplicate scopes")
	}
}

func TestIsSubsetSameScope(t *testing.T) {
	a := mustParse(t, "s3:read:bucket/incident-42")
	if !IsSubset(a, a) {
		t.Fatalf("%q should be a subset of itself", a)
	}
}

func TestIsSubsetNarrowerResource(t *testing.T) {
	parent := mustParse(t, "s3:read:bucket/incident-42")
	child := mustParse(t, "s3:read:bucket/incident-42/logs")
	if !IsSubset(child, parent) {
		t.Fatalf("%q should be a subset of %q", child, parent)
	}
}

func TestIsSubsetDifferentAction(t *testing.T) {
	parent := mustParse(t, "s3:read:bucket/incident-42")
	child := mustParse(t, "s3:write:bucket/incident-42")
	if IsSubset(child, parent) {
		t.Fatalf("%q should not be a subset of %q", child, parent)
	}
}

func TestIsSetSubsetAllCovered(t *testing.T) {
	parent, err := ParseAll([]string{
		"s3:read:bucket/incident-42",
		"s3:list:bucket/incident-42",
	})
	if err != nil {
		t.Fatal(err)
	}
	child, err := ParseAll([]string{"s3:read:bucket/incident-42/logs"})
	if err != nil {
		t.Fatal(err)
	}
	if !IsSetSubset(child, parent) {
		t.Fatal("child set should be covered by parent set")
	}
}

func TestIsSetSubsetMissingScope(t *testing.T) {
	parent, err := ParseAll([]string{"s3:read:bucket/incident-42"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := ParseAll([]string{"s3:read:bucket/incident-42", "s3:delete:bucket/incident-42"})
	if err != nil {
		t.Fatal(err)
	}
	if IsSetSubset(child, parent) {
		t.Fatal("child set with an extra action should not be covered")
	}
	missing := Missing(child, parent)
	if len(missing) != 1 || missing[0].Action != "s3:delete" {
		t.Fatalf("Missing() = %v", missing)
	}
}
