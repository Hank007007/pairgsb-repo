# sidemount design notes

This note describes the shape of the sidecar and the environment it assumes.
The behaviour it must exhibit is specified in [mount-rules.md](mount-rules.md);
this document does not restate that contract and does not prescribe how the
contract is met.

## Role in the pod

```
        +-------------------+        HTTP(S)        +-----------------+
        |    sidemount      | <--------------------> | credential      |
        |    sidecar        |                        | broker          |
        +---------+---------+                        +-----------------+
                  |
                  | writes
                  v
        /run/secrets/<name>   (shared volume)
                  ^
                  | read-only
        +---------+---------+
        | workload process  |
        +-------------------+
```

The sidecar holds the identity material needed to talk to the broker. The
workload never sees that material and never needs broker credentials; it only
reads files from the shared mount.

## Components

| Module | Responsibility |
| --- | --- |
| `src/cli.js` | Entry point. Validates configuration, obtains the first bundle, starts the command loop and lease timer. |
| `src/config.js` | Reads configuration from the environment and validates it. |
| `src/broker.js` | Minimal HTTP client for the broker: fetch a bundle for a service, renew a lease. |
| `src/names.js` | Decides whether a broker-supplied name is admissible. |
| `src/mount.js` | Target directory ownership: materialize a secret set and remove materialized files. |
| `src/renew.js` | Applies a renewed bundle to an existing session. |
| `src/daemon.js` | stdin command loop, self-renewal timer, signal handling, shutdown sequencing. |

## Configuration

| Variable | Meaning | Default |
| --- | --- | --- |
| `SIDEMOUNT_SERVICE` | Service identity presented to the broker | required |
| `SIDEMOUNT_BROKER_URL` | Base URL of the broker | `http://127.0.0.1:8200` |
| `SIDEMOUNT_IDENTITY_TOKEN` | Bearer token for broker calls | empty |
| `SIDEMOUNT_SECRETS_DIR` | Directory that receives the files | `/run/secrets` |
| `SIDEMOUNT_LEASE_SECONDS` | Fallback lease lifetime when the broker grants none | `300` |

## Broker protocol

`sidemount` expects two endpoints:

- `GET /v1/secrets/<service>` — initial bundle.
- `POST /v1/leases/<leaseId>/renew` — renewed bundle.

Both answer with:

```json
{
  "leaseId": "lease-42",
  "ttlSeconds": 600,
  "secrets": { "db_password": "...", "api_key": "..." }
}
```

`leaseId` and `secrets` are required; secret values are strings. A non-2xx
status, an unparseable body, or an unexpected shape is a failed request.

## Lifecycle

1. Configuration is validated; an invalid configuration terminates the
   process before anything is written.
2. The target directory is prepared and the first bundle is applied.
3. A command loop reads stdin (`renew`, `stop`) while a timer watches the lease.
4. `stop`, `SIGTERM`, or stdin EOF leads to the shutdown sequence.
5. Shutdown reports, removes what the sidecar placed, and exits.

A renewal failure is reported and does not take the process down; the mounted
set stays as it was until a later renewal succeeds.

## Running locally

The repository ships a broker stub so the sidecar can be exercised without a
real credential service:

```sh
node tools/mock-broker.js --bundle tools/bundle.sample.json --port 8200
```

In a second terminal:

```sh
SIDEMOUNT_SERVICE=demo-service \
SIDEMOUNT_BROKER_URL=http://127.0.0.1:8200 \
SIDEMOUNT_SECRETS_DIR=$PWD/.run/secrets \
node src/cli.js
```

Then type `renew` or `stop` on stdin. Edit `tools/bundle.sample.json` between
renewals to change the secret set. Note that the default target is
`/run/secrets`; point `SIDEMOUNT_SECRETS_DIR` elsewhere when that path is not
available or not writable.

## Tests

```sh
node --test
```

The suite covers the normal path: mounting a bundle, renewing contents, and
exiting on `stop`. It uses an in-process broker stub and temporary
directories, so it needs no privileged setup.
