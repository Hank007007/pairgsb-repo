# pathvault access model

pathvault is an internal broker that hands secrets to service processes. A caller
presents an accessor token, names a path such as `kv/app/payments/stripe`, and the
broker either returns the secret or refuses. Every successful hand-out is recorded
as a lease so that access can be revoked per backend and audited per path.

This document is the specification. It describes observable behaviour only; it
does not describe how the code is organised.

## 1. Roles and tokens

There are two kinds of token.

- **Accessor token** — a long-lived bearer token issued to a service. It
  authenticates the caller and carries the accessor identity and the policy rules
  attached to it. Accessor tokens are read from the policy files at start-up
  (§9.2) and never change while the process runs.
- **Wrap token** — a short-lived, single-use token minted by the broker from an
  accessor token and a path (§6). It can be exchanged exactly once for the secret
  at that path.

Tokens are opaque strings. They must only travel in the `Authorization: Bearer`
header or in the request body of the unwrap endpoint, never in a path segment.

## 2. Secret paths

A path is a sequence of one or more non-empty segments separated by `/`, for
example `kv/app/payments/stripe`. Paths are case-sensitive, have no trailing
slash, and are matched in full: a pattern must cover the entire path.

Every secret is stored with a **backend** name. The backend is a property of the
secret record, not of the path: several paths may be served by the same backend,
and a backend is the unit at which cached material is revoked (§5).

## 3. Policy

An accessor carries an ordered list of rules. Each rule has an effect (`allow` or
`deny`) and a path pattern.

### 3.1 Pattern matching

Patterns use the same segment syntax as paths with two wildcards:

- `*` matches **exactly one** segment. It never crosses a `/`.
- `**` matches **zero or more** segments, and is only valid as a whole segment.

Examples:

| Pattern | Matches | Does not match |
|---|---|---|
| `kv/app/*` | `kv/app/db` | `kv/app/db/prod`, `kv/app` |
| `kv/app/**` | `kv/app`, `kv/app/db`, `kv/app/db/prod` | `kv/other/db` |
| `kv/app/payments/*` | `kv/app/payments/stripe` | `kv/app/payments/stripe/key` |
| `kv/*/stripe` | `kv/app/stripe` | `kv/app/payments/stripe` |

Matching is anchored at both ends. Literal characters in the pattern match
themselves; there is no other metacharacter.

### 3.2 Rule evaluation

For a given accessor and path:

1. If **any** matching rule has effect `deny`, the request is denied. Deny always
   wins over allow, regardless of the order in which the rules appear.
2. Otherwise, if at least one matching rule has effect `allow`, the request is
   allowed.
3. Otherwise the request is denied.

The order of rules in a policy file has no effect on the outcome. Policies are
written in YAML-like reading order for humans only.

## 4. Reads and leases

### 4.1 Direct reads

`GET /v1/secret/{path}` with an accessor token:

- unknown or malformed accessor token → `401`;
- policy denies the path, or no secret exists at that path → `404`;
- otherwise the broker returns the secret and creates a lease.

Every successful read creates a lease. A lease is bound to the accessor, the
path, and the backend that serves the secret. The response contains the lease
identifier.

### 4.2 Lease reads

A client that already holds a lease can repeat the read by sending the lease
identifier in the `X-Lease-Id` header:

`GET /v1/secret/{path}` with `Authorization: Bearer <accessor>` and
`X-Lease-Id: <lease>`.

The lease must exist, must not have been revoked, and must have been issued to
the same accessor for the same path. If any of those checks fail the read is
refused with `403`. Lease reads do not re-run policy: authorisation happened
when the lease was created. Lease reads also do not create a new lease.

### 4.3 Lease lifetime

Leases do not expire on their own. They remain valid until the backend they are
bound to is revoked (§5) or the process exits. A lease is never re-bound to a
different backend or path.

## 5. Backend revocation

`POST /v1/backends/{backend}/revoke` (admin token required) revokes a backend
instance. Revocation is by backend name, not by lease or by path.

A revocation must:

1. mark **every** lease whose backend is the named backend as revoked, not only
   the most recent one;
2. invalidate all secret material cached for that backend, so no lease issued for
   it can serve a value afterwards.

After revocation, a read that presents any lease belonging to the revoked backend
must fail with `403`, even if that lease was issued seconds earlier and is still
"current" from the client's point of view. Leases belonging to other backends,
and the secrets of other backends, must be unaffected.

Revocation is idempotent. Revoking a backend with no leases is not an error; the
response reports how many leases were revoked. A subsequent direct read of a path
on the revoked backend is a fresh authorisation: if policy still allows it, it
succeeds and creates a new lease.

## 6. Wrap tokens

`POST /v1/wrap` with an accessor token, a path, and an optional `ttl` in
**seconds** mints a wrap token. The accessor must be allowed to read the path, or
the request fails with `404`.

Rules:

- The default TTL, used when `ttl` is absent or not positive, is 300 seconds.
- A wrap token is valid for the requested number of seconds after it is minted.
- A wrap token is **single-use**. `POST /v1/unwrap` exchanges it for the secret,
  the broker creates a lease exactly as for a direct read, and the wrap token is
  consumed. A second unwrap of the same token must fail with `404`, even if it
  happens immediately after the first.
- An expired token must also fail with `404`.
- Consumed and expired tokens are indistinguishable from unknown tokens to the
  caller. The broker may audit the failed attempt but must not return secret
  material and must not create a lease.

`ttl` is measured in seconds: `{"ttl": 60}` means the token is valid for one
minute, not for a moment and not until the process restarts.

## 7. Audit

Every read, wrap, unwrap, revocation, and refusal is appended to an audit log as
one JSON object per line (JSONL). Each event records at least:

- `time` — RFC 3339 timestamp with sub-second precision;
- `accessor` — the accessor identity, or `""` if authentication failed;
- `path` — the requested path;
- `action` — one of `read`, `wrap`, `unwrap`, `revoke`;
- `decision` — one of `allow`, `deny`, `error`;
- `lease_id` and `backend` when they apply.

Audit records must contain **no secret material**. The secret value, and any
derived or obfuscated form of it, must never be written to the audit log. The
audit log is safe to ship to a log aggregator: the only sensitive identifiers it
contains are the accessor identity and the path, and those are exactly what
incident response needs to reconstruct a read.

A read must remain discoverable by accessor: given an accessor identity and a
path, the log contains an `allow` event with the lease that was issued.

## 8. HTTP API

All bodies are JSON. Errors are `{"error": "<message>"}`.

| Method | Path | Auth | Success |
|---|---|---|---|
| `GET` | `/healthz` | none | `200 {"status":"ok"}` |
| `GET` | `/v1/secret/{path}` | accessor | `200 {"path","value","lease_id","backend"}` |
| `POST` | `/v1/wrap` | accessor | `201 {"token","expires_at"}` |
| `POST` | `/v1/unwrap` | wrap token in body | `200 {"path","value","lease_id","backend"}` |
| `POST` | `/v1/backends/{backend}/revoke` | admin | `200 {"backend","revoked"}` |

Request bodies:

```json
{"path": "kv/app/payments/stripe", "ttl": 60}
```

is the wrap request; `ttl` is optional. The unwrap request body is
`{"token": "wrap_..."}`.

Status codes:

- `401` — missing, unknown, or malformed accessor/admin token.
- `403` — revoked lease.
- `404` — policy denial, unknown secret path, unknown/expired/consumed wrap
  token, or unknown backend on revocation.
- `405` — wrong method for the route.

A policy denial deliberately looks like a missing secret so that callers cannot
probe for the existence of paths.

## 9. Configuration

### 9.1 Secrets

The secret table is loaded once at start-up from a JSON file:

```json
{
  "secrets": [
    {"path": "kv/app/payments/stripe", "value": "sk_live_...", "backend": "payments-vault"}
  ]
}
```

`path`, `value`, and `backend` are all required and must be non-empty. Duplicate
paths are a configuration error and the process must refuse to start. The file is
read only; the in-memory table is replaced on restart.

### 9.2 Policies

Policies are loaded once at start-up from one or more JSON files with the same
shape:

```json
{
  "accessors": [
    {
      "id": "svc-payments",
      "token": "acc_payments_...",
      "rules": [
        {"effect": "allow", "path": "kv/app/payments/*"},
        {"effect": "deny",  "path": "kv/app/payments/stripe/rotate"}
      ]
    }
  ]
}
```

`id` and `token` are required and unique. An unknown `effect`, an empty `path`,
or a duplicate `id`/`token` is a configuration error. Rule order is preserved
for readability but has no semantic effect (§3.2).
