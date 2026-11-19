package tuple

import "testing"

func mustObject(t *testing.T, s string) Object {
	t.Helper()
	obj, err := ParseObject(s)
	if err != nil {
		t.Fatalf("ParseObject(%q): %v", s, err)
	}
	return obj
}

func mustSubject(t *testing.T, s string) Subject {
	t.Helper()
	sub, err := ParseSubject(s)
	if err != nil {
		t.Fatalf("ParseSubject(%q): %v", s, err)
	}
	return sub
}

func TestStoreRevisionAdvances(t *testing.T) {
	store := NewStore()
	doc := mustObject(t, "document:doc-1")
	alice := mustSubject(t, "user:alice")

	if got := store.Revision(); got != 0 {
		t.Fatalf("initial revision = %d, want 0", got)
	}

	rev, err := store.Write(doc, "viewer", alice)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if rev != 1 {
		t.Errorf("Write revision = %d, want 1", rev)
	}

	bob := mustSubject(t, "user:bob")
	rev, err = store.Write(doc, "viewer", bob)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if rev != 2 {
		t.Errorf("Write revision = %d, want 2", rev)
	}

	rev, err = store.Delete(doc, "viewer", alice)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if rev != 3 {
		t.Errorf("Delete revision = %d, want 3", rev)
	}
	if got := store.Revision(); got != 3 {
		t.Errorf("store revision = %d, want 3", got)
	}
}

func TestStoreWriteIsIdempotent(t *testing.T) {
	store := NewStore()
	doc := mustObject(t, "document:doc-1")
	alice := mustSubject(t, "user:alice")

	first, err := store.Write(doc, "viewer", alice)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	second, err := store.Write(doc, "viewer", alice)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if first != second {
		t.Errorf("duplicate write moved revision from %d to %d", first, second)
	}
}

func TestStoreDeleteMissingIsIdempotent(t *testing.T) {
	store := NewStore()
	doc := mustObject(t, "document:doc-1")
	alice := mustSubject(t, "user:alice")

	rev, err := store.Delete(doc, "viewer", alice)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if rev != 0 {
		t.Errorf("deleting a missing tuple moved revision to %d", rev)
	}
}

func TestStoreValidation(t *testing.T) {
	store := NewStore()
	doc := mustObject(t, "document:doc-1")
	alice := mustSubject(t, "user:alice")

	if _, err := store.Write(Object{}, "viewer", alice); err == nil {
		t.Error("Write with empty object succeeded")
	}
	if _, err := store.Write(doc, "", alice); err == nil {
		t.Error("Write with empty relation succeeded")
	}
	if _, err := store.Write(doc, "viewer", Subject{}); err == nil {
		t.Error("Write with empty subject succeeded")
	}
}

func TestSnapshotSubjects(t *testing.T) {
	store := NewStore()
	doc := mustObject(t, "document:doc-1")
	other := mustObject(t, "document:doc-2")
	alice := mustSubject(t, "user:alice")
	bob := mustSubject(t, "user:bob")

	if _, err := store.Write(doc, "viewer", alice); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Write(doc, "viewer", bob); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Write(doc, "editor", bob); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Write(other, "viewer", alice); err != nil {
		t.Fatal(err)
	}

	snapshot := store.Latest()
	viewers := snapshot.Subjects(doc, "viewer")
	if len(viewers) != 2 {
		t.Fatalf("viewer count = %d, want 2", len(viewers))
	}
	if !snapshot.HasRelation(doc, "editor") {
		t.Error("expected editor relation on doc")
	}
	if snapshot.HasRelation(other, "editor") {
		t.Error("unexpected editor relation on other")
	}
	if got := snapshot.Relations(doc); len(got) != 2 || got[0] != "editor" || got[1] != "viewer" {
		t.Errorf("Relations(doc) = %v", got)
	}
	if got := snapshot.Objects(); len(got) != 2 {
		t.Errorf("Objects() = %v", got)
	}
}

func TestSnapshotIsImmutable(t *testing.T) {
	store := NewStore()
	doc := mustObject(t, "document:doc-1")
	alice := mustSubject(t, "user:alice")
	bob := mustSubject(t, "user:bob")

	if _, err := store.Write(doc, "viewer", alice); err != nil {
		t.Fatal(err)
	}
	before := store.Latest()
	if _, err := store.Write(doc, "viewer", bob); err != nil {
		t.Fatal(err)
	}

	if got := len(before.Subjects(doc, "viewer")); got != 1 {
		t.Errorf("earlier snapshot changed: viewer count = %d", got)
	}
	if got := len(store.Latest().Subjects(doc, "viewer")); got != 2 {
		t.Errorf("latest snapshot viewer count = %d, want 2", got)
	}
	clone := before.Clone()
	if got := len(clone.Subjects(doc, "viewer")); got != 1 {
		t.Errorf("cloned snapshot viewer count = %d", got)
	}
}

func TestSnapshotAtEarlierRevision(t *testing.T) {
	store := NewStore()
	doc := mustObject(t, "document:doc-1")
	alice := mustSubject(t, "user:alice")
	bob := mustSubject(t, "user:bob")

	if _, err := store.Write(doc, "viewer", alice); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Write(doc, "viewer", bob); err != nil {
		t.Fatal(err)
	}

	snapshot, err := store.View(1)
	if err != nil {
		t.Fatalf("View(1): %v", err)
	}
	if snapshot.Revision() != 1 {
		t.Errorf("snapshot revision = %d, want 1", snapshot.Revision())
	}
	viewers := snapshot.Subjects(doc, "viewer")
	if len(viewers) != 1 || viewers[0] != alice {
		t.Errorf("viewers at revision 1 = %v", viewers)
	}
}

func TestSnapshotViewErrors(t *testing.T) {
	store := NewStore()
	doc := mustObject(t, "document:doc-1")
	alice := mustSubject(t, "user:alice")
	bob := mustSubject(t, "user:bob")
	if _, err := store.Write(doc, "viewer", alice); err != nil {
		t.Fatal(err)
	}

	if _, err := store.View(2); err != ErrFutureRevision {
		t.Errorf("View(2) error = %v, want ErrFutureRevision", err)
	}
	if _, err := store.View(1); err != nil {
		t.Errorf("View(1) error = %v", err)
	}

	if err := store.CollectGarbage(1); err != nil {
		t.Fatalf("CollectGarbage: %v", err)
	}
	if _, err := store.View(1); err != nil {
		t.Errorf("View(1) after collection at floor 1: %v", err)
	}
	if _, err := store.Write(doc, "viewer", bob); err != nil {
		t.Fatal(err)
	}
	if err := store.CollectGarbage(2); err != nil {
		t.Fatalf("CollectGarbage: %v", err)
	}
	if _, err := store.View(1); err != ErrStaleRevision {
		t.Errorf("View(1) after collection at floor 2: %v, want ErrStaleRevision", err)
	}
}

func TestCollectGarbageBoundaries(t *testing.T) {
	store := NewStore()
	if err := store.CollectGarbage(1); err == nil {
		t.Error("CollectGarbage beyond current revision succeeded")
	}
	if got := store.Revision(); got != 0 {
		t.Errorf("revision changed by failed collection: %d", got)
	}
}

func TestSnapshotEmptyStore(t *testing.T) {
	store := NewStore()
	snapshot := store.Latest()
	if snapshot.Revision() != 0 {
		t.Errorf("empty store revision = %d", snapshot.Revision())
	}
	if len(snapshot.Objects()) != 0 {
		t.Errorf("empty store has objects: %v", snapshot.Objects())
	}
}
