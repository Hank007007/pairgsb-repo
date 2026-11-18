// Command permcache exercises the decision cache against a small built-in
// policy set and prints one line of JSON per request.
//
// Run it with:
//
//	go run ./cmd/permcache
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"permcache/internal/decide"
	"permcache/internal/policy"
)

const bundleJSON = `{
  "revision": "9",
  "rules": [
    {
      "id": "allow-alice-docs",
      "effect": "allow",
      "tenant": "blue",
      "subject": "user:alice",
      "action": "docs.*",
      "resource": "doc:*"
    },
    {
      "id": "deny-secret-docs",
      "effect": "deny",
      "tenant": "blue",
      "subject": "user:*",
      "action": "docs.*",
      "resource": "secret:*"
    },
    {
      "id": "allow-bob-anywhere",
      "effect": "allow",
      "tenant": "*",
      "subject": "user:bob",
      "action": "*",
      "resource": "*"
    }
  ]
}`

const upgradeJSON = `{
  "revision": "10",
  "rules": [
    {
      "id": "deny-alice-docs",
      "effect": "deny",
      "tenant": "blue",
      "subject": "user:alice",
      "action": "docs.*",
      "resource": "doc:*"
    },
    {
      "id": "allow-bob-anywhere",
      "effect": "allow",
      "tenant": "*",
      "subject": "user:bob",
      "action": "*",
      "resource": "*"
    }
  ]
}`

type output struct {
	Label       string `json:"label"`
	Tenant      string `json:"tenant"`
	Subject     string `json:"subject"`
	Action      string `json:"action"`
	Resource    string `json:"resource"`
	Effect      string `json:"effect,omitempty"`
	MatchedRule string `json:"matched_rule,omitempty"`
	Revision    string `json:"revision,omitempty"`
	Cached      bool   `json:"cached"`
	Error       string `json:"error,omitempty"`
}

func emit(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}
}

func run(engine *decide.Engine, label string, req decide.Request) {
	out := output{
		Label:    label,
		Tenant:   req.Tenant,
		Subject:  req.Subject,
		Action:   req.Action,
		Resource: req.Resource,
	}
	d, err := engine.Decide(req)
	if err != nil {
		out.Error = err.Error()
		emit(out)
		return
	}
	out.Effect = string(d.Effect)
	out.MatchedRule = d.MatchedRule
	out.Revision = d.Revision
	out.Cached = d.Cached
	emit(out)
}

func report(engine *decide.Engine, note string) {
	stats := engine.Store().Stats()
	fmt.Fprintf(os.Stderr, "%-34s entries=%d hits=%d misses=%d evictions=%d hit_ratio=%.2f\n",
		note, stats.Entries, stats.Hits, stats.Misses, stats.Evictions, stats.HitRatio)
}

func main() {
	loader := policy.NewLoader()
	if err := loader.LoadJSON([]byte(bundleJSON)); err != nil {
		fmt.Fprintf(os.Stderr, "load bundle: %v\n", err)
		os.Exit(1)
	}

	engine := decide.New(loader)
	engine.SetTTL(30 * time.Second)

	attrs := map[string]string{
		"region": "eu",
		"tier":   "gold",
		"device": "laptop",
		"mfa":    "true",
	}

	run(engine, "alice-docs-read", decide.Request{
		Tenant: "blue", Subject: "user:alice", Action: "docs.read", Resource: "doc:1",
	})
	run(engine, "alice-docs-read-again", decide.Request{
		Tenant: "blue", Subject: "user:alice", Action: "docs.read", Resource: "doc:1",
	})
	run(engine, "alice-docs-read-private", decide.Request{
		Tenant: "blue", Subject: "user:alice", Action: "docs.read.private", Resource: "secret:1",
	})
	run(engine, "alice-docs-delete", decide.Request{
		Tenant: "blue", Subject: "user:alice", Action: "docs.delete", Resource: "doc:1",
	})
	run(engine, "alice-docs-read-green", decide.Request{
		Tenant: "green", Subject: "user:alice", Action: "docs.read", Resource: "doc:1",
	})
	for i := 0; i < 3; i++ {
		run(engine, fmt.Sprintf("alice-with-attrs-%d", i+1), decide.Request{
			Tenant: "blue", Subject: "user:alice", Action: "docs.read", Resource: "doc:7",
			Attributes: attrs,
		})
	}
	report(engine, "after baseline traffic")

	if err := loader.LoadJSON([]byte(upgradeJSON)); err != nil {
		emit(output{Label: "load-revision-10", Error: err.Error()})
	} else {
		rev, _ := loader.Revision()
		emit(output{Label: "load-revision-10", Revision: rev})
	}
	run(engine, "alice-docs-read-after-upgrade", decide.Request{
		Tenant: "blue", Subject: "user:alice", Action: "docs.read", Resource: "doc:1",
	})

	removed, err := engine.ApplyRevocation(decide.Revocation{
		Tenant: "*", Subject: "user:alice", Action: "*", Resource: "*",
	})
	if err != nil {
		emit(output{Label: "revoke-alice", Error: err.Error()})
	} else {
		emit(map[string]any{"label": "revoke-alice", "removed": removed})
	}
	run(engine, "alice-docs-read-after-revoke", decide.Request{
		Tenant: "blue", Subject: "user:alice", Action: "docs.read", Resource: "doc:1",
	})

	report(engine, "after upgrade and revocation")
}
