# peerloom

`peerloom` is a small peer-authentication and authorization library for
mutually authenticated RPC services. It is deliberately dependency-free: only
the Go standard library is used.

A peer presents a client certificate whose URI SAN follows the SPIFFE naming
shape:

```
spiffe://<trust-domain>/<service>/<instance>
```

The library extracts a service identity from that certificate, then evaluates
the call tables under `policies/*.json` to decide whether a method may be
invoked. Trust anchors and intermediate CAs live under `internal/bundle/` and
are rotated by epoch. The local trust domain is declared in
`policies/trust.json`.

## Layout

| Path | Purpose |
|---|---|
| `docs/peer-authz.md` | Normative design: handshake, identity, policy matching, deny precedence, call coverage |
| `docs/incident-reports.md` | Operational and red-team incident write-ups that inform the design |
| `policies/` | Sample trust domain and call tables |
| `internal/bundle/` | Trust anchor and intermediate CA layout, rotated by epoch |
| `internal/identity/` | Peer identity model and certificate extraction |
| `internal/policy/` | Call-table loading and evaluation |
| `internal/server/` | Server-side TLS configuration |
| `internal/rpc/` | Request dispatch and authorization for unary and streaming calls |
| `internal/wire/` | Length-prefixed framing shared by the two commands |
| `internal/certtest/` | Certificate helpers used by the tests |
| `cmd/echo-peer/` | Minimal echoing RPC server |
| `cmd/call-peer/` | Minimal client used to drive `echo-peer` |
| `cmd/devcerts/` | Writes a local CA, server certificate and client certificates |

## Build and test

```sh
go build ./...
go test ./...
```

## Local reproduction

`cmd/devcerts` writes a root CA, an intermediate CA, a server certificate for
`localhost` and one client certificate per URI SAN:

```sh
go run ./cmd/devcerts \
  -out devcerts \
  -client "spiffe://corp/payments/i-local,spiffe://corp/reporting/i-local"
```

Pass `-bundle-dir internal/bundle` to also install the root CA and intermediate
CA as the anchors of the epoch named by `-epoch`, rewriting `epochs.json`. That
gives `echo-peer` trust material straight from the bundle:

```sh
go run ./cmd/devcerts -out devcerts -bundle-dir internal/bundle \
  -client "spiffe://corp/payments/i-local"
```

Then run a server and a client in separate terminals:

```sh
go run ./cmd/echo-peer \
  -listen 127.0.0.1:8443 \
  -cert devcerts/server.pem -key devcerts/server-key.pem \
  -ca "" \
  -bundle ./internal/bundle \
  -policies ./policies \
  -v

go run ./cmd/call-peer \
  -addr 127.0.0.1:8443 \
  -cert devcerts/client-0.pem -key devcerts/client-0-key.pem \
  -ca devcerts/ca.pem -server-name localhost \
  -method /ledger.v1.Ledger/Debit \
  -body 'amount=42'
```

Pass `-stream` to `call-peer` to exercise the streaming dispatch path and
`-count` to send more than one message over the stream.

`echo-peer -ca` takes a plain PEM file of client CAs. When it is empty the
server falls back to the anchors and intermediate CAs of the trust bundle.

On Windows shells that rewrite leading slashes on arguments, quote the method
(`"-method=/ledger.v1.Ledger/Debit"`) or set `MSYS_NO_PATHCONV=1`; otherwise the
client sends a mangled path.

The trust bundle directory must follow the layout documented in
`internal/bundle/README.md`.
