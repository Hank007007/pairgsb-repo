# signward

A signing proxy for build machines. Build steps never see key material: they
hold an opaque key handle and ask the agent for an Ed25519 signature over a
payload. Keys live in the agent process and are persisted only inside the
agent's own key directory.

The rules the agent must follow — key lifecycle, purpose allow-lists, hourly
signature quota, payload limits, session lifetime and the audit trail — are
written down in [`docs/agent-policy.md`](docs/agent-policy.md). That document
is normative.

## Layout

| Path | Contents |
|---|---|
| `cmd/signward/main.go` | process entry point, flags, line protocol |
| `internal/agent/agent.go` | the public signing entry point: `Unlock`, `Lock`, `Sign` |
| `internal/policy/policy.go` | per-key limits and the hourly counter |
| `internal/keystore/store.go` | key generation, handles, persistence, lifecycle |
| `internal/audit/log.go` | append-only JSON-lines audit trail |

## Building and testing

```
go build ./...
go test ./...
```

Only the Go standard library is used; no network access is required and the
tests generate their keys in temporary directories.

## Running

```
go run ./cmd/signward -key-dir ./signward-keys -audit-log ./signward-keys/audit.log
```

The process reads one JSON request per line on stdin and writes one JSON
reply per line on stdout.

```json
{"op":"create","id":"release","policy":{"purposes":["sign"],"maxSignaturesPerHour":100,"maxPayloadBytes":4096}}
{"op":"unlock"}
{"op":"sign","keyHandle":"kh_...","purpose":"sign","payload":"<base64>"}
{"op":"destroy","keyHandle":"kh_..."}
{"op":"lock"}
```

Payloads and signatures are base64. Replies look like
`{"ok":true,"signature":"<base64>"}` or `{"ok":false,"error":"..."}`.
