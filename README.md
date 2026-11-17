# sidemount

An injection sidecar for containers. It pulls a set of secrets from a
credential broker on behalf of a service identity, materializes them under
`/run/secrets/<name>`, and keeps them current while the workload runs.

The behaviour contract is [`docs/mount-rules.md`](docs/mount-rules.md). The
module layout and local run instructions are in
[`docs/design.md`](docs/design.md).

## Quick start

```sh
# terminal 1 — broker stub
node tools/mock-broker.js --bundle tools/bundle.sample.json --port 8200

# terminal 2 — sidecar
export SIDEMOUNT_SERVICE=demo-service
export SIDEMOUNT_BROKER_URL=http://127.0.0.1:8200
export SIDEMOUNT_SECRETS_DIR="$PWD/.run/secrets"
node src/cli.js
# then type: renew / stop
```

## Configuration

| Variable | Meaning | Default |
| --- | --- | --- |
| `SIDEMOUNT_SERVICE` | Service identity presented to the broker | required |
| `SIDEMOUNT_BROKER_URL` | Broker base URL | `http://127.0.0.1:8200` |
| `SIDEMOUNT_IDENTITY_TOKEN` | Bearer token for broker calls | empty |
| `SIDEMOUNT_SECRETS_DIR` | Directory receiving the files | `/run/secrets` |
| `SIDEMOUNT_LEASE_SECONDS` | Fallback lease lifetime | `300` |

## stdin commands

- `renew` — fetch and apply a fresh bundle.
- `stop` — shut down.

## Tests

```sh
node --test
```

Requires Node.js 20 or newer. No third-party dependencies.
