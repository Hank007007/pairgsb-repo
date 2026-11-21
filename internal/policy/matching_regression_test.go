package policy_test

import (
	"testing"

	"maskseal-journal/internal/policy"
)

func match(t *testing.T, doc, path string) (policy.Action, bool) {
	t.Helper()
	rs, err := policy.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return rs.Match(path)
}

func TestMatchIsExactPerSegment(t *testing.T) {
	doc := `{"rules":[{"path":"user.password","action":"redact"}]}`
	if action, matched := match(t, doc, "user.password_hint"); matched || action != policy.ActionRedact {
		t.Fatalf("user.password_hint matched by user.password: action=%q matched=%v", action, matched)
	}
	if action, matched := match(t, doc, "userXpassword"); matched || action != policy.ActionRedact {
		t.Fatalf("userXpassword matched by user.password: action=%q matched=%v", action, matched)
	}
	if action, matched := match(t, doc, "user.password"); !matched || action != policy.ActionRedact {
		t.Fatalf("user.password did not match: action=%q matched=%v", action, matched)
	}
}

func TestMatchArrayWildcard(t *testing.T) {
	doc := `{"rules":[{"path":"items[].ssn","action":"redact"}]}`
	for _, path := range []string{"items[0].ssn", "items[1].ssn", "items[42].ssn"} {
		if action, matched := match(t, doc, path); !matched || action != policy.ActionRedact {
			t.Fatalf("%s did not match items[].ssn: action=%q matched=%v", path, action, matched)
		}
	}
	for _, path := range []string{"items[0].nested.ssn", "items.ssn", "items[0]"} {
		if _, matched := match(t, doc, path); matched {
			t.Fatalf("%s unexpectedly matched items[].ssn", path)
		}
	}
}

func TestMatchCaseInsensitiveASCIIOnly(t *testing.T) {
	doc := `{"rules":[{"path":"User.Password","action":"redact"}]}`
	if action, matched := match(t, doc, "user.password"); !matched || action != policy.ActionRedact {
		t.Fatalf("case-insensitive match failed: action=%q matched=%v", action, matched)
	}
	if _, matched := match(t, doc, "user.pässword"); matched {
		t.Fatal("non-ASCII characters must compare exactly")
	}
	if _, matched := match(t, `{"rules":[{"path":"user.pässword","action":"redact"}]}`, "user.PÄSSWORD"); matched {
		t.Fatal("non-ASCII case folding must not occur")
	}
}

func TestUnmatchedFieldsDefaultToRedact(t *testing.T) {
	doc := `{"rules":[{"path":"source","action":"keep"}]}`
	if action, matched := match(t, doc, "user.name"); matched || action != policy.ActionRedact {
		t.Fatalf("unmatched field action=%q matched=%v, want default redact", action, matched)
	}
	if action, matched := match(t, doc, "source"); !matched || action != policy.ActionKeep {
		t.Fatalf("explicit keep failed: action=%q matched=%v", action, matched)
	}
}
