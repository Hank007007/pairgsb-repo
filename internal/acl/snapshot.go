package acl

import "sync"

// Snapshot is the pinned view that a single evaluation works against.
//
// All lookups made through a Snapshot observe the same generation of the
// security groups, the group membership directory and the prefix lists, no
// matter how many of them happen during the evaluation.
type Snapshot struct {
	directory        *GroupDirectory
	prefixes         map[string]PrefixList
	groups           map[string]SecurityGroup
	directoryVer     int
	prefixCatalogVer int
}

// SnapshotSource hands out a pinned view for one evaluation.
type SnapshotSource interface {
	Snapshot() *Snapshot
}

// Versions reports the generation of the membership directory and of the
// prefix catalog captured in the snapshot.
func (s *Snapshot) Versions() (groups int, prefixes int) {
	if s == nil {
		return 0, 0
	}
	return s.directoryVer, s.prefixCatalogVer
}

// SecurityGroup returns a copy of the named group as it stood when the
// snapshot was taken.
func (s *Snapshot) SecurityGroup(id string) (SecurityGroup, bool) {
	if s == nil {
		return SecurityGroup{}, false
	}
	sg, ok := s.groups[id]
	if !ok {
		return SecurityGroup{}, false
	}
	return cloneSecurityGroup(sg), true
}

// GroupMembers returns the instances that resolve a reference to groupID in
// the captured membership generation.
func (s *Snapshot) GroupMembers(groupID string) []Member {
	if s == nil {
		return nil
	}
	return s.directory.MembersOf(groupID)
}

// PrefixList returns the captured version of the named list.
func (s *Snapshot) PrefixList(name string) (PrefixList, bool) {
	if s == nil {
		return PrefixList{}, false
	}
	list, ok := s.prefixes[name]
	if !ok {
		return PrefixList{}, false
	}
	return list.Clone(), true
}

// Registry owns the live policy inputs. Every mutation is serialised so that
// a snapshot observes one coherent generation across all of them.
type Registry struct {
	mu        sync.RWMutex
	directory *GroupDirectory
	prefixes  *PrefixCatalog
	groups    map[string]SecurityGroup
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		directory: NewGroupDirectory(),
		prefixes:  NewPrefixCatalog(),
		groups:    make(map[string]SecurityGroup),
	}
}

// SetGroup installs or replaces a security group.
func (r *Registry) SetGroup(sg SecurityGroup) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.groups[sg.ID] = cloneSecurityGroup(sg)
}

// Group returns the current definition of a security group.
func (r *Registry) Group(id string) (SecurityGroup, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sg, ok := r.groups[id]
	if !ok {
		return SecurityGroup{}, false
	}
	return cloneSecurityGroup(sg), true
}

// Attach records that an instance belongs to a group.
func (r *Registry) Attach(m Member) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.directory.Attach(m)
}

// Detach removes an instance from the membership directory.
func (r *Registry) Detach(instanceID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.directory.Detach(instanceID)
}

// SetPrefixList replaces the entries of a prefix list.
func (r *Registry) SetPrefixList(name string, entries ...string) (PrefixList, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.prefixes.Put(name, entries...)
}

// MembershipVersion reports the current generation of the membership
// directory.
func (r *Registry) MembershipVersion() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.directory.Version()
}

// PrefixVersion reports the current generation of the prefix catalog.
func (r *Registry) PrefixVersion() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.prefixes.Version()
}

// Snapshot captures all policy inputs together.
func (r *Registry) Snapshot() *Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snap := &Snapshot{
		directory:        r.directory.Clone(),
		prefixes:         r.prefixes.Snapshot(),
		groups:           make(map[string]SecurityGroup, len(r.groups)),
		directoryVer:     r.directory.Version(),
		prefixCatalogVer: r.prefixes.Version(),
	}
	for id, sg := range r.groups {
		snap.groups[id] = cloneSecurityGroup(sg)
	}
	return snap
}

func cloneSecurityGroup(sg SecurityGroup) SecurityGroup {
	out := SecurityGroup{ID: sg.ID}
	if sg.Ingress != nil {
		out.Ingress = make(RuleBook, len(sg.Ingress))
		for number, rule := range sg.Ingress {
			out.Ingress[number] = rule
		}
	}
	if sg.Egress != nil {
		out.Egress = make(RuleBook, len(sg.Egress))
		for number, rule := range sg.Egress {
			out.Egress[number] = rule
		}
	}
	return out
}
