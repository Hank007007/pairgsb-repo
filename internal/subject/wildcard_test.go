package subject

import (
	"testing"

	"tuplegrate/internal/tuple"
)

func mustSubject(t *testing.T, s string) tuple.Subject {
	t.Helper()
	sub, err := tuple.ParseSubject(s)
	if err != nil {
		t.Fatalf("ParseSubject(%q): %v", s, err)
	}
	return sub
}

func TestMatchExact(t *testing.T) {
	alice := mustSubject(t, "user:alice")
	if !Match(alice, alice) {
		t.Error("terminal subject did not match itself")
	}
	if Match(alice, mustSubject(t, "user:bob")) {
		t.Error("distinct terminal subjects matched")
	}
	if Match(alice, mustSubject(t, "service:alice")) {
		t.Error("subjects in different namespaces matched")
	}
}

func TestMatchSubjectSet(t *testing.T) {
	eng := mustSubject(t, "group:eng#member")
	if !Match(eng, eng) {
		t.Error("subject set did not match itself")
	}
	if Match(eng, mustSubject(t, "group:ops#member")) {
		t.Error("distinct subject sets matched")
	}
	if Match(eng, mustSubject(t, "group:eng")) {
		t.Error("subject set matched a terminal subject")
	}
	if Match(mustSubject(t, "group:eng"), eng) {
		t.Error("terminal subject matched a subject set")
	}
}

func TestMatchWildcardSameNamespace(t *testing.T) {
	wildcard := mustSubject(t, "user:*")
	for _, requested := range []string{"user:alice", "user:bob", "user:ci-bot"} {
		if !Match(wildcard, mustSubject(t, requested)) {
			t.Errorf("user:* did not match %s", requested)
		}
	}
	if Match(wildcard, mustSubject(t, "service:ci-bot")) {
		t.Error("user:* matched a subject in another namespace")
	}
}

func TestMatchWildcardIsNotUniversal(t *testing.T) {
	if Match(mustSubject(t, "user:alice"), mustSubject(t, "user:*")) {
		t.Error("terminal subject matched a wildcard request")
	}
}

func TestExpand(t *testing.T) {
	object, relation, ok := Expand(mustSubject(t, "group:eng#member"))
	if !ok || relation != "member" {
		t.Fatalf("Expand = %v, %q, %v", object, relation, ok)
	}
	if object.Namespace != "group" || object.ID != "eng" {
		t.Errorf("expanded object = %+v", object)
	}
	if _, _, ok := Expand(mustSubject(t, "user:alice")); ok {
		t.Error("terminal subject reported as expandable")
	}
	if _, _, ok := Expand(mustSubject(t, "user:*")); ok {
		t.Error("wildcard subject reported as expandable")
	}
}

func TestNamespaceOf(t *testing.T) {
	if got := NamespaceOf(mustSubject(t, "group:eng#member")); got != "group" {
		t.Errorf("NamespaceOf = %q", got)
	}
}
