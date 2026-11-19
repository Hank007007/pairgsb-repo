package acl

import (
	"net/netip"
)

// Member is one instance that currently belongs to a security group.
type Member struct {
	InstanceID string
	GroupID    string
	Address    netip.Addr
}

// GroupDirectory is the live view of which instances belong to which security
// groups. Group references are resolved against it.
type GroupDirectory struct {
	version int
	members []Member
}

// NewGroupDirectory builds a directory from the given members.
func NewGroupDirectory(members ...Member) *GroupDirectory {
	d := &GroupDirectory{version: 1}
	d.members = append(d.members, members...)
	return d
}

// Version reports the generation of the directory. It is bumped on every
// membership change.
func (d *GroupDirectory) Version() int {
	if d == nil {
		return 0
	}
	return d.version
}

// AllMembers returns a copy of every member in the directory.
func (d *GroupDirectory) AllMembers() []Member {
	if d == nil {
		return nil
	}
	out := make([]Member, len(d.members))
	copy(out, d.members)
	return out
}

// Attach records that an instance belongs to a group.
func (d *GroupDirectory) Attach(m Member) {
	d.members = append(d.members, m)
	d.version++
}

// Detach removes an instance from the directory. It reports whether the
// instance was present.
func (d *GroupDirectory) Detach(instanceID string) bool {
	for i, m := range d.members {
		if m.InstanceID == instanceID {
			d.members = append(d.members[:i], d.members[i+1:]...)
			d.version++
			return true
		}
	}
	return false
}

// MembersOf returns the instances that resolve a reference to groupID.
func (d *GroupDirectory) MembersOf(groupID string) []Member {
	if d == nil {
		return nil
	}
	var out []Member
	for _, m := range d.members {
		if m.GroupID == groupID {
			out = append(out, m)
		}
	}
	return out
}

// Clone returns an independent copy of the directory at its current version.
func (d *GroupDirectory) Clone() *GroupDirectory {
	if d == nil {
		return NewGroupDirectory()
	}
	c := &GroupDirectory{version: d.version}
	c.members = make([]Member, len(d.members))
	copy(c.members, d.members)
	return c
}
