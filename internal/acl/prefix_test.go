package acl

import (
	"net/netip"
	"testing"
)

func TestContainsIPv4InsideAndOutside(t *testing.T) {
	entries := []string{"10.0.0.0/8", "192.168.4.0/24"}
	if !Contains(entries, netip.MustParseAddr("10.9.8.7")) {
		t.Fatal("10.9.8.7 was not matched by 10.0.0.0/8")
	}
	if !Contains(entries, netip.MustParseAddr("192.168.4.200")) {
		t.Fatal("192.168.4.200 was not matched by 192.168.4.0/24")
	}
	if Contains(entries, netip.MustParseAddr("172.16.0.1")) {
		t.Fatal("172.16.0.1 matched a list that does not contain it")
	}
}

func TestContainsIPv6Outside(t *testing.T) {
	entries := []string{"2001:db8::/32"}
	if !Contains(entries, netip.MustParseAddr("2001:db8::1")) {
		t.Fatal("2001:db8::1 was not matched")
	}
	if Contains(entries, netip.MustParseAddr("2001:db9::1")) {
		t.Fatal("2001:db9::1 matched 2001:db8::/32")
	}
}

func TestContainsSkipsMalformedEntries(t *testing.T) {
	entries := []string{"not-a-cidr", "10.0.0.0/8"}
	if !Contains(entries, netip.MustParseAddr("10.1.1.1")) {
		t.Fatal("a malformed entry prevented a later valid entry from matching")
	}
}

func TestCatalogPutAndGet(t *testing.T) {
	cat := NewPrefixCatalog()
	first, err := cat.Put("corp", "10.0.0.0/8")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if first.Version == 0 {
		t.Fatal("first version is zero")
	}
	got, ok := cat.Get("corp")
	if !ok {
		t.Fatal("Get did not find the stored list")
	}
	if !got.Contains(netip.MustParseAddr("10.2.3.4")) {
		t.Fatal("stored list does not contain a member address")
	}
}

func TestCatalogPutRejectsMalformedCIDR(t *testing.T) {
	cat := NewPrefixCatalog()
	if _, err := cat.Put("bad", "10.0.0.0/33"); err == nil {
		t.Fatal("Put accepted an invalid prefix")
	}
	if _, ok := cat.Get("bad"); ok {
		t.Fatal("a rejected Put created a list")
	}
}

func TestCatalogVersionAdvances(t *testing.T) {
	cat := NewPrefixCatalog()
	start := cat.Version()
	if _, err := cat.Put("corp", "10.0.0.0/8"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if cat.Version() == start {
		t.Fatal("catalog version did not advance")
	}
}

func TestCatalogSnapshotIsIndependent(t *testing.T) {
	cat := NewPrefixCatalog()
	if _, err := cat.Put("corp", "10.0.0.0/8"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	snap := cat.Snapshot()
	if _, err := cat.Put("corp", "192.168.0.0/16"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	pinned := snap["corp"]
	if !pinned.Contains(netip.MustParseAddr("10.5.5.5")) {
		t.Fatal("snapshot lost the original entries")
	}
	if pinned.Contains(netip.MustParseAddr("192.168.5.5")) {
		t.Fatal("snapshot observed a later Put")
	}
}
