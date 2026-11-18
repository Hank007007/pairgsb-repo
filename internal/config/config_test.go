package config

import (
	"strings"
	"testing"
)

func TestParseAppliesDefaults(t *testing.T) {
	cfg, err := Parse([]byte(`{
		"routes": [
			{"name": "api", "prefix": "/api", "upstream": "http://127.0.0.1:9001"}
		]
	}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Listen != DefaultListen {
		t.Errorf("Listen = %q, want %q", cfg.Listen, DefaultListen)
	}
	if cfg.TimeoutSeconds != DefaultTimeoutSeconds {
		t.Errorf("TimeoutSeconds = %d, want %d", cfg.TimeoutSeconds, DefaultTimeoutSeconds)
	}
}

func TestParseValidDocument(t *testing.T) {
	cfg, err := Parse([]byte(`{
		"listen": "127.0.0.1:8080",
		"timeout_seconds": 5,
		"routes": [
			{
				"name": "admin",
				"prefix": "/admin",
				"upstream": "http://127.0.0.1:9002",
				"allow": ["10.0.0.0/8", "127.0.0.1"],
				"deny": ["10.9.0.0/16"],
				"deny_paths": ["/admin/ops"]
			}
		]
	}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cfg.Routes) != 1 {
		t.Fatalf("routes = %d, want 1", len(cfg.Routes))
	}
	allow, err := ParseCIDRs(cfg.Routes[0].Allow)
	if err != nil {
		t.Fatalf("ParseCIDRs: %v", err)
	}
	if len(allow) != 2 {
		t.Fatalf("allow prefixes = %d, want 2", len(allow))
	}
	if allow[1].Bits() != 32 || allow[1].Addr().String() != "127.0.0.1" {
		t.Errorf("plain address parsed as %v, want 127.0.0.1/32", allow[1])
	}
}

func TestParseRejectsInvalidDocuments(t *testing.T) {
	cases := map[string]string{
		"unknown field": `{
			"routes": [{"name": "api", "prefix": "/api", "upstream": "http://127.0.0.1:9001"}],
			"surprise": true
		}`,
		"missing routes": `{"listen": "127.0.0.1:8080"}`,
		"bad prefix": `{
			"routes": [{"name": "api", "prefix": "api", "upstream": "http://127.0.0.1:9001"}]
		}`,
		"bad upstream scheme": `{
			"routes": [{"name": "api", "prefix": "/api", "upstream": "ftp://127.0.0.1:9001"}]
		}`,
		"bad cidr": `{
			"routes": [{"name": "api", "prefix": "/api", "upstream": "http://127.0.0.1:9001", "allow": ["10.0.0.0/64"]}]
		}`,
		"duplicate name": `{
			"routes": [
				{"name": "api", "prefix": "/api", "upstream": "http://127.0.0.1:9001"},
				{"name": "api", "prefix": "/v2", "upstream": "http://127.0.0.1:9002"}
			]
		}`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(doc)); err == nil {
				t.Fatal("Parse succeeded, want error")
			}
		})
	}
}

func TestParseRejectsTrailingData(t *testing.T) {
	doc := `{"routes":[{"name":"api","prefix":"/api","upstream":"http://127.0.0.1:9001"}]} {}`
	if _, err := Parse([]byte(doc)); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("Parse error = %v, want trailing data error", err)
	}
}
