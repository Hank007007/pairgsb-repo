package check

import (
	"testing"

	"tuplegrate/internal/schema"
	"tuplegrate/internal/tuple"
)

const baseSchema = `
type group {
    relation member = self
}

type folder {
    relation parent = self
    relation editor = self
    relation viewer = self | editor
}

type document {
    relation parent = self
    relation editor = self
    relation viewer = self | editor
}
`

type fixture struct {
	t      *testing.T
	store  *tuple.Store
	svc    *Service
	schema *schema.Schema
}

func newFixture(t *testing.T, text string) *fixture {
	t.Helper()
	rules, err := schema.Parse(text)
	if err != nil {
		t.Fatalf("schema.Parse: %v", err)
	}
	store := tuple.NewStore()
	return &fixture{t: t, store: store, svc: New(store, rules), schema: rules}
}

func (f *fixture) write(object, relation, subject string) {
	f.t.Helper()
	obj, err := tuple.ParseObject(object)
	if err != nil {
		f.t.Fatalf("ParseObject(%q): %v", object, err)
	}
	sub, err := tuple.ParseSubject(subject)
	if err != nil {
		f.t.Fatalf("ParseSubject(%q): %v", subject, err)
	}
	if _, err := f.store.Write(obj, relation, sub); err != nil {
		f.t.Fatalf("Write(%s#%s@%s): %v", object, relation, subject, err)
	}
}

func (f *fixture) remove(object, relation, subject string) {
	f.t.Helper()
	obj, err := tuple.ParseObject(object)
	if err != nil {
		f.t.Fatalf("ParseObject(%q): %v", object, err)
	}
	sub, err := tuple.ParseSubject(subject)
	if err != nil {
		f.t.Fatalf("ParseSubject(%q): %v", subject, err)
	}
	if _, err := f.store.Delete(obj, relation, sub); err != nil {
		f.t.Fatalf("Delete(%s#%s@%s): %v", object, relation, subject, err)
	}
}

func (f *fixture) check(object, relation, subject string) Result {
	f.t.Helper()
	res, err := f.svc.Check(Request{Subject: subject, Relation: relation, Object: object})
	if err != nil {
		f.t.Fatalf("Check(%s, %s, %s): %v", subject, relation, object, err)
	}
	return res
}

func (f *fixture) checkAt(object, relation, subject string, revision tuple.Revision) (Result, error) {
	f.t.Helper()
	return f.svc.Check(Request{Subject: subject, Relation: relation, Object: object, Revision: revision})
}

func (f *fixture) wantAllowed(object, relation, subject string, want bool) {
	f.t.Helper()
	res := f.check(object, relation, subject)
	if res.Allowed != want {
		f.t.Errorf("Check(%s, %s, %s).Allowed = %v, want %v", subject, relation, object, res.Allowed, want)
	}
}

func TestCheckSelf(t *testing.T) {
	f := newFixture(t, baseSchema)
	f.write("document:doc-1", "viewer", "user:alice")

	f.wantAllowed("document:doc-1", "viewer", "user:alice", true)
	f.wantAllowed("document:doc-1", "viewer", "user:bob", false)
	f.wantAllowed("document:doc-2", "viewer", "user:alice", false)
}

func TestCheckUnion(t *testing.T) {
	f := newFixture(t, baseSchema)
	f.write("document:doc-1", "editor", "user:alice")
	f.write("document:doc-1", "viewer", "user:bob")

	f.wantAllowed("document:doc-1", "viewer", "user:alice", true)
	f.wantAllowed("document:doc-1", "viewer", "user:bob", true)
	f.wantAllowed("document:doc-1", "viewer", "user:carol", false)
}

func TestCheckIntersection(t *testing.T) {
	f := newFixture(t, `
type document {
    relation editor = self
    relation viewer = self
    relation both = editor & viewer
}
`)
	f.write("document:doc-1", "editor", "user:alice")
	f.write("document:doc-1", "editor", "user:bob")
	f.write("document:doc-1", "viewer", "user:alice")

	f.wantAllowed("document:doc-1", "both", "user:alice", true)
	f.wantAllowed("document:doc-1", "both", "user:bob", false)
}

func TestCheckSubjectSetExpansion(t *testing.T) {
	f := newFixture(t, baseSchema)
	f.write("document:doc-1", "viewer", "group:eng#member")
	f.write("group:eng", "member", "user:alice")
	f.write("group:ops", "member", "user:bob")

	f.wantAllowed("document:doc-1", "viewer", "user:alice", true)
	f.wantAllowed("document:doc-1", "viewer", "user:bob", false)
}

func TestCheckNestedSubjectSet(t *testing.T) {
	f := newFixture(t, baseSchema)
	f.write("document:doc-1", "viewer", "group:eng#member")
	f.write("group:eng", "member", "group:sub#member")
	f.write("group:sub", "member", "user:alice")

	f.wantAllowed("document:doc-1", "viewer", "user:alice", true)
}

func TestCheckFromSingleHop(t *testing.T) {
	f := newFixture(t, `
type folder {
    relation viewer = self
}

type document {
    relation viewer = self | viewer from viewer
}
`)
	f.write("document:doc-1", "viewer", "folder:eng")
	f.write("folder:eng", "viewer", "user:alice")

	f.wantAllowed("document:doc-1", "viewer", "user:alice", true)
	f.wantAllowed("document:doc-1", "viewer", "user:bob", false)
}

func TestCheckShallowNesting(t *testing.T) {
	f := newFixture(t, `
type group {
    relation member = self
}

type folder {
    relation viewer = self
}

type document {
    relation viewer = self | viewer from viewer
}
`)
	f.write("document:doc-1", "viewer", "folder:eng")
	f.write("folder:eng", "viewer", "group:eng#member")
	f.write("group:eng", "member", "user:alice")

	f.wantAllowed("document:doc-1", "viewer", "user:alice", true)
}

func TestCheckWildcard(t *testing.T) {
	f := newFixture(t, baseSchema)
	f.write("document:pub", "viewer", "user:*")

	f.wantAllowed("document:pub", "viewer", "user:alice", true)
	f.wantAllowed("document:pub", "viewer", "user:bob", true)
	f.wantAllowed("document:pub", "viewer", "service:ci", false)
}

func TestCheckRequestValidation(t *testing.T) {
	f := newFixture(t, baseSchema)

	tests := []struct {
		name string
		req  Request
		code Code
	}{
		{
			name: "bad subject",
			req:  Request{Subject: "alice", Relation: "viewer", Object: "document:doc-1"},
			code: CodeInvalidRequest,
		},
		{
			name: "bad object",
			req:  Request{Subject: "user:alice", Relation: "viewer", Object: "document"},
			code: CodeInvalidRequest,
		},
		{
			name: "empty relation",
			req:  Request{Subject: "user:alice", Relation: "", Object: "document:doc-1"},
			code: CodeInvalidRequest,
		},
		{
			name: "unknown namespace",
			req:  Request{Subject: "user:alice", Relation: "viewer", Object: "project:p-1"},
			code: CodeUnknownRelation,
		},
		{
			name: "unknown relation",
			req:  Request{Subject: "user:alice", Relation: "owner", Object: "document:doc-1"},
			code: CodeUnknownRelation,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.svc.Check(tc.req)
			if err == nil {
				t.Fatalf("Check succeeded, want %s", tc.code)
			}
			if got := CodeOf(err); got != tc.code {
				t.Errorf("code = %q, want %q (err: %v)", got, tc.code, err)
			}
		})
	}
}

func TestCheckResultRevision(t *testing.T) {
	f := newFixture(t, baseSchema)
	f.write("document:doc-1", "viewer", "user:alice")
	f.write("document:doc-1", "viewer", "user:bob")

	res := f.check("document:doc-1", "viewer", "user:alice")
	if res.Revision != 2 {
		t.Errorf("revision = %d, want 2", res.Revision)
	}
}

func TestCheckEmptyStore(t *testing.T) {
	f := newFixture(t, baseSchema)
	f.wantAllowed("document:doc-1", "viewer", "user:alice", false)
}

func TestCheckUsesSchemaRelations(t *testing.T) {
	f := newFixture(t, baseSchema)
	if names := f.svc.Schema().Names(); len(names) != 3 {
		t.Errorf("schema names = %v", names)
	}
	view := f.svc.Latest()
	if view == nil {
		t.Fatal("Latest returned nil")
	}
	if view.Revision() != 0 {
		t.Errorf("empty view revision = %d", view.Revision())
	}
}
