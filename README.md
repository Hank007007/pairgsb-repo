# pathvault

An internal broker that hands secrets to service processes under a path policy,
issues leases on every hand-out, and writes an audit trail. The access model,
including pattern semantics, wrap-token rules, and revocation behaviour, is
specified in [docs/access-model.md](docs/access-model.md).

## Layout

```
cmd/pathvaultd/          server entry point
internal/policy/         pattern matching, rule evaluation, policy loading
internal/token/          accessor and wrap tokens
internal/lease/          leases and backend revocation
internal/store/          in-memory secret table
internal/api/            HTTP routes and request handling
internal/audit/          JSONL audit log
testdata/                sample secret and policy files
```

## Running

```sh
go run ./cmd/pathvaultd \
  -secrets testdata/secrets.json \
  -policy testdata/policy-payments.json \
  -policy testdata/policy-ops.json \
  -addr 127.0.0.1:8080 \
  -audit /tmp/pathvault-audit.jsonl \
  -admin-token adm_test
```

Flags:

| Flag | Meaning |
|---|---|
| `-secrets` | JSON file with the secret table (required) |
| `-policy` | policy file; repeatable |
| `-addr` | listen address (default `127.0.0.1:8080`) |
| `-audit` | audit log path; empty means stdout |
| `-admin-token` | token accepted for backend revocation |

## Trying it

The sample policy files carry fixed tokens so the examples below work out of
the box.

```sh
# allowed read
curl -s -H 'Authorization: Bearer acc_payments_dev' \
  http://127.0.0.1:8080/v1/secret/kv/app/payments/stripe

# mint a wrap token valid for one minute
curl -s -X POST -H 'Authorization: Bearer acc_payments_dev' \
  -d '{"path":"kv/app/payments/stripe","ttl":60}' \
  http://127.0.0.1:8080/v1/wrap

# exchange it
curl -s -X POST -d '{"token":"wrap_..."}' http://127.0.0.1:8080/v1/unwrap

# revoke a backend
curl -s -X POST -H 'Authorization: Bearer adm_test' \
  http://127.0.0.1:8080/v1/backends/payments-core/revoke
```

## Tests

```sh
go test ./...
```

The HTTP-level tests drive the full request path with `httptest`.
