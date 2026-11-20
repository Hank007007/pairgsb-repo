# certwhirl

certwhirl is a small certificate steward for long-running mTLS services. It

- reads a leaf certificate and private key from disk,
- publishes them as an atomically replaceable `tls.Config`,
- replaces the pair before it expires, and
- builds matching client and server TLS configurations that share one
  certificate authority pool.

Only the Go standard library is used.

## Layout

```
cmd/certwhirld/     demo daemon that serves TLS from a managed pair
internal/loader/    LoadPair: read, parse and check certificate material
internal/rotator/   Rotator: initial load, background loop, replacement
internal/tlspool/   BuildClientTLS / BuildServerTLS
internal/testca/    self-signed CA helper used by tests and fixtures
testdata/           PEM fixtures (good, rogue, misnamed, near-expiry, expired)
docs/               rotation policy and peer verification notes
tools/genfixtures/  maintenance tool that regenerated testdata/
```

## Quick start

```sh
go test ./...

go run ./cmd/certwhirld \
  -cert testdata/good/leaf.pem \
  -key testdata/good/leaf-key.pem \
  -ca testdata/good/ca.pem \
  -identity service.internal \
  -addr 127.0.0.1:8443
```

Then connect with a client that trusts `testdata/good/ca.pem`:

```sh
openssl s_client -connect 127.0.0.1:8443 \
  -CAfile testdata/good/ca.pem \
  -servername service.internal
```

## Policies

Two documents describe the behaviour the packages are expected to provide:

- [`docs/rotation-policy.md`](docs/rotation-policy.md)
- [`docs/peer-verify.md`](docs/peer-verify.md)

Read them alongside the code; they are the specification for the loader,
rotator and tlspool packages.

## Test fixtures

`testdata/` ships static PEM files with dates far from the current day:

| directory     | contents                                                       |
|---------------|----------------------------------------------------------------|
| `good/`       | leaf valid to 2035 for `service.internal`, `localhost`, 127.0.0.1, plus a client leaf |
| `rogue/`      | leaf for `rogue.example.com` issued by an unrelated CA          |
| `misnamed/`   | leaf for `impostor.internal` issued by the `good` CA            |
| `near-expiry/`| leaf for `service.internal` expiring soon after 2035-01-01      |
| `expired/`    | leaf that expired at the start of 2024                          |

`tools/genfixtures` regenerates these files; it is excluded from normal
builds.
