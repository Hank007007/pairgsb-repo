package tenant_test

import (
	"testing"

	"topicwarden/internal/tenant"
)

func TestNormalizeTopicKeepsLayout(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"tenant/acme/sensors/a", "tenant/acme/sensors/a"},
		{"tenant/acme/sensors/a/", "tenant/acme/sensors/a/"},
		{"tenant/acme/a//b", "tenant/acme/a//b"},
	}
	for _, c := range cases {
		if got := tenant.NormalizeTopic(c.in); got != c.want {
			t.Errorf("NormalizeTopic(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTenantOf(t *testing.T) {
	if got := tenant.TenantOf("tenant/acme/sensors/a"); got != "acme" {
		t.Fatalf("TenantOf = %q, want acme", got)
	}
	if got := tenant.TenantOf("sensors/a"); got != "" {
		t.Fatalf("TenantOf plain topic = %q, want empty", got)
	}
}

func TestStripPrefix(t *testing.T) {
	if got := tenant.StripPrefix("tenant/acme/sensors/a"); got != "sensors/a" {
		t.Fatalf("StripPrefix = %q, want sensors/a", got)
	}
}
