package acl

import (
	"testing"
)

func TestRegistrySnapshotPinsVersions(t *testing.T) {
	reg := NewRegistry()
	reg.SetGroup(SecurityGroup{
		ID: "sg-app",
		Ingress: NewRuleBook(
			Rule{Number: 100, Action: ActionAllow, Protocol: ProtocolTCP, FromPort: 443, ToPort: 443, Source: PrefixListSource("corp")},
		),
	})
	if _, err := reg.SetPrefixList("corp", "10.0.0.0/8"); err != nil {
		t.Fatalf("SetPrefixList: %v", err)
	}

	snap := reg.Snapshot()
	groupsVer, prefixesVer := snap.Versions()
	if groupsVer != reg.MembershipVersion() {
		t.Fatalf("snapshot membership version %d, registry %d", groupsVer, reg.MembershipVersion())
	}
	if prefixesVer != reg.PrefixVersion() {
		t.Fatalf("snapshot prefix version %d, registry %d", prefixesVer, reg.PrefixVersion())
	}

	// Changes made after the snapshot are invisible to it.
	if _, err := reg.SetPrefixList("corp", "192.168.0.0/16"); err != nil {
		t.Fatalf("SetPrefixList: %v", err)
	}
	reg.Attach(member("i-001", "sg-app", "10.0.0.9"))

	pinned, ok := snap.PrefixList("corp")
	if !ok {
		t.Fatal("snapshot lost the prefix list")
	}
	if !pinned.Contains(mustAddr(t, "10.0.0.5")) {
		t.Fatal("snapshot prefix list does not hold the version captured at Snapshot time")
	}
	if pinned.Contains(mustAddr(t, "192.168.0.5")) {
		t.Fatal("snapshot prefix list observed a change made after Snapshot")
	}
	if got := len(snap.GroupMembers("sg-app")); got != 0 {
		t.Fatalf("snapshot membership observed an instance attached after Snapshot: %d", got)
	}
}

func TestSnapshotSecurityGroupIsACopy(t *testing.T) {
	reg := NewRegistry()
	reg.SetGroup(SecurityGroup{
		ID: "sg-app",
		Ingress: NewRuleBook(
			Rule{Number: 100, Action: ActionAllow, Protocol: ProtocolTCP, FromPort: 443, ToPort: 443, Source: CIDRSource("10.0.0.0/8")},
		),
	})
	snap := reg.Snapshot()
	sg, ok := snap.SecurityGroup("sg-app")
	if !ok {
		t.Fatal("snapshot does not hold the group")
	}
	sg.Ingress[100] = Rule{Number: 100, Action: ActionDeny}

	again, _ := snap.SecurityGroup("sg-app")
	if again.Ingress[100].Action != ActionAllow {
		t.Fatal("mutating a returned group changed the snapshot")
	}
	current, _ := reg.Group("sg-app")
	if current.Ingress[100].Action != ActionAllow {
		t.Fatal("mutating a returned group changed the registry")
	}
}
