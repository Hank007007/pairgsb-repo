package cache

import "testing"

func TestBuildKeyDistinguishesSubjectsAndResources(t *testing.T) {
	a := BuildKey("tenant-a", "user:alice", "read", "doc:1", nil, "1")
	b := BuildKey("tenant-a", "user:bob", "read", "doc:1", nil, "1")
	c := BuildKey("tenant-a", "user:alice", "read", "doc:2", nil, "1")
	d := BuildKey("tenant-a", "user:alice", "read", "doc:12", nil, "1")

	if a == b {
		t.Fatal("different subjects share a key")
	}
	if a == c {
		t.Fatal("different resources share a key")
	}
	if c == d {
		t.Fatal("doc:2 and doc:12 share a key")
	}
	if a != BuildKey("tenant-a", "user:alice", "read", "doc:1", nil, "1") {
		t.Fatal("identical inputs produced different keys")
	}
}

func TestBuildKeyIncludesRevision(t *testing.T) {
	a := BuildKey("tenant-a", "user:alice", "read", "doc:1", nil, "1")
	b := BuildKey("tenant-a", "user:alice", "read", "doc:1", nil, "2")
	if a == b {
		t.Fatal("different revisions share a key")
	}
}

func TestFingerprintAttributes(t *testing.T) {
	attrs := map[string]string{"level": "3"}
	got := fingerprint(attrs)
	if got == "-" {
		t.Fatal("fingerprint of a populated map is empty")
	}
	if got != fingerprint(map[string]string{"level": "3"}) {
		t.Fatal("fingerprint is not reproducible")
	}
	if fingerprint(nil) != fingerprint(map[string]string{}) {
		t.Fatal("nil and empty attribute maps disagree")
	}
	if fingerprint(map[string]string{"a": "1"}) == fingerprint(map[string]string{"a": "2"}) {
		t.Fatal("different attribute values share a fingerprint")
	}
}
