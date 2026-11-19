# egressvault

egressvault is the egress-side credential injection proxy for the platform. A
business service sends ordinary HTTP requests to egressvault; egressvault
looks up the credential that belongs to the target host, writes it into the
upstream request, forwards over TLS, and relays the response. The business
process never holds the credential.

```
business ──HTTP──▶ egressvault ──TLS──▶ upstream
                        │
                        ─ vault file  (credentials + host rules)
                        └─ audit log   (one JSON record per hop)
```

## Documents

| file | purpose |
|---|---|
| `docs/egress-rules.md` | normative contract from the security and network groups |
| `docs/design.md` | design notes: request lifecycle, boundaries, configuration, testing |

The rules document wins whenever the two disagree.

## Layout

```
src/index.js      process entry point
src/proxy.js      admission, hop construction, transport, response relay
src/routing.js    host → rule matching, NO_PROXY matching
src/redirect.js   3xx classification, Location resolution, next-hop construction
src/tls.js        upstream TLS parameters and client certificate selection
src/vault.js      vault file parsing and credential material
src/audit.js      per-hop audit records
test/             node:test suites against loopback http/https upstreams
test/fixtures/    vault sample, CA, server and client certificates
```

No runtime dependencies beyond the Node.js standard library.

## Requirements

Node.js 20 or newer. `node:crypto`'s `X509Certificate` is used by the test
suite to verify the fixtures.

## Running

```sh
export EGRESS_VAULT_FILE="$PWD/test/fixtures/vault.json"
export EGRESS_AUDIT_FILE="$PWD/audit/egress.jsonl"
export EGRESS_CA_FILE="$PWD/test/fixtures/ca.pem"   # only for the fixture CA
export EGRESS_PORT=8080
node src/index.js
```

Then point a caller at it:

```sh
curl -x http://127.0.0.1:8080 https://localhost:8443/health
```

The startup line reports the bound address and how many rules were loaded. A
vault that fails validation stops the process with exit code 2 before any
socket is opened.

## Configuration

See §9 of `docs/design.md` and §7 of `docs/egress-rules.md`. In short:
`EGRESS_VAULT_FILE`, `EGRESS_AUDIT_FILE`, `EGRESS_MAX_REDIRECTS`,
`EGRESS_CA_FILE`, `EGRESS_BIND`, `EGRESS_PORT`, `EGRESS_TIMEOUT_MS`, plus the
usual `HTTP_PROXY` / `HTTPS_PROXY` / `ALL_PROXY` / `NO_PROXY` family for an
upstream proxy.

## Vault file

```json
{
  "credentials": {
    "svc-example-api": {
      "scheme": "Bearer",
      "secret": "…",
      "headers": { "X-Egress-Tenant": "example" }
    },
    "svc-example-internal": {
      "header": "X-Api-Key",
      "secret": "…"
    }
  },
  "rules": [
    { "host": "api.example.com", "credential": "svc-example-api" },
    { "host": "internal.example.com", "credential": "svc-example-internal" }
  ]
}
```

A credential uses either the default `Authorization: <scheme> <secret>` form or
a custom `header` carrying the secret alone. `clientCert` (with `certFile`,
`keyFile` and an optional `passphrase`) selects a client certificate for that
credential's hops; paths are relative to wherever the vault file lives.

Validation is strict on purpose. Unknown credential references, duplicate rule
hosts, malformed header names and unreadable client certificate material are
start-up errors, not warnings.

## Tests

```sh
node --test
```

The suite starts local `http` and `https` servers on loopback and drives the
proxy with absolute-form requests, so it never leaves the machine. The fixture
certificates are issued by `test/fixtures/ca.pem`; pass it as `EGRESS_CA_FILE`
when exercising the fixture HTTPS upstream by hand.
