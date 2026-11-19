package acl

import (
	"net/netip"
	"testing"
)

func member(instanceID, groupID, addr string) Member {
	return Member{InstanceID: instanceID, GroupID: groupID, Address: netip.MustParseAddr(addr)}
}

func TestMembersOfExactGroup(t *testing.T) {
	dir := NewGroupDirectory(
		member("i-001", "sg-web", "10.0.1.10"),
		member("i-002", "sg-web", "10.0.1.11"),
		member("i-003", "sg-db", "10.0.2.20"),
	)
	got := dir.MembersOf("sg-web")
	if len(got) != 2 {
		t.Fatalf("MembersOf(sg-web) returned %d members, want 2", len(got))
	}
	for _, m := range got {
		if m.GroupID != "sg-web" {
			t.Fatalf("MembersOf(sg-web) returned a member of %q", m.GroupID)
		}
	}
}

func TestMembersOfUnknownGroup(t *testing.T) {
	dir := NewGroupDirectory(member("i-001", "sg-web", "10.0.1.10"))
	if got := dir.MembersOf("sg-nope"); len(got) != 0 {
		t.Fatalf("MembersOf(sg-nope) returned %d members, want 0", len(got))
	}
}

func TestDirectoryVersionAdvances(t *testing.T) {
	dir := NewGroupDirectory()
	start := dir.Version()
	dir.Attach(member("i-010", "sg-web", "10.0.1.20"))
	if dir.Version() == start {
		t.Fatal("version did not advance after Attach")
	}
	beforeDetach := dir.Version()
	if !dir.Detach("i-010") {
		t.Fatal("Detach did not find an attached instance")
	}
	if dir.Version() == beforeDetach {
		t.Fatal("version did not advance after Detach")
	}
	if dir.Detach("i-010") {
		t.Fatal("Detach reported success for an absent instance")
	}
}

func TestCloneIsIndependent(t *testing.T) {
	dir := NewGroupDirectory(member("i-001", "sg-web", "10.0.1.10"))
	c := dir.Clone()
	if c.Version() != dir.Version() {
		t.Fatalf("clone version %d, original version %d", c.Version(), dir.Version())
	}
	c.Attach(member("i-002", "sg-web", "10.0.1.11"))
	if got := len(dir.AllMembers()); got != 1 {
		t.Fatalf("original directory changed after mutating the clone: %d members", got)
	}
}
