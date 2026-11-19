package acl

import (
	"fmt"
	"net/netip"
)

// PrefixList is a named, versioned set of CIDR blocks.
type PrefixList struct {
	Name    string
	Version int
	Entries []string
}

// Clone returns an independent copy of the list.
func (p PrefixList) Clone() PrefixList {
	c := PrefixList{Name: p.Name, Version: p.Version}
	c.Entries = make([]string, len(p.Entries))
	copy(c.Entries, p.Entries)
	return c
}

// Contains reports whether ip falls inside any entry of the list.
func (p PrefixList) Contains(ip netip.Addr) bool {
	return Contains(p.Entries, ip)
}

// Contains reports whether ip falls inside any of the given CIDR blocks.
func Contains(entries []string, ip netip.Addr) bool {
	ip = ip.Unmap()
	for _, raw := range entries {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			continue
		}
		if prefix.Addr().Is4In6() {
			bits := prefix.Bits() - 96
			if bits < 0 || bits > 32 {
				continue
			}
			prefix = netip.PrefixFrom(prefix.Addr().Unmap(), bits).Masked()
		}
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

// PrefixCatalog holds the current version of every prefix list.
type PrefixCatalog struct {
	version int
	lists   map[string]PrefixList
}

// NewPrefixCatalog returns an empty catalog.
func NewPrefixCatalog() *PrefixCatalog {
	return &PrefixCatalog{lists: make(map[string]PrefixList)}
}

// Version reports the generation of the catalog. It is bumped on every
// change to any list.
func (c *PrefixCatalog) Version() int {
	if c == nil {
		return 0
	}
	return c.version
}

// Put replaces the entries of the named list, creating it when necessary.
// Every entry must be a valid CIDR block.
func (c *PrefixCatalog) Put(name string, entries ...string) (PrefixList, error) {
	for _, raw := range entries {
		if _, err := netip.ParsePrefix(raw); err != nil {
			return PrefixList{}, fmt.Errorf("acl: prefix list %q: %w", name, err)
		}
	}
	c.version++
	next := c.version
	if existing, ok := c.lists[name]; ok {
		next = existing.Version + 1
	}
	list := PrefixList{Name: name, Version: next, Entries: append([]string(nil), entries...)}
	c.lists[name] = list
	return list.Clone(), nil
}

// Get returns the current version of a list.
func (c *PrefixCatalog) Get(name string) (PrefixList, bool) {
	if c == nil {
		return PrefixList{}, false
	}
	list, ok := c.lists[name]
	if !ok {
		return PrefixList{}, false
	}
	return list.Clone(), true
}

// Snapshot returns an independent copy of every list currently known to the
// catalog.
func (c *PrefixCatalog) Snapshot() map[string]PrefixList {
	out := make(map[string]PrefixList)
	if c == nil {
		return out
	}
	for name, list := range c.lists {
		out[name] = list.Clone()
	}
	return out
}
