# scopewall

`scopewall` is the authorization decision point (PDP) that sits between services in the mesh.
Every internal call passes through an HTTP middleware, which hands the caller identity, the
resource path and the action to the policy engine. The engine turns a policy document plus a
delegation graph into an allow/deny decision, and decisions are memoized in a bounded
TTL cache.

The normative behavior of the access model lives in [`docs/access-model.md`](docs/access-model.md).
That document is the contract; the code is expected to implement it.

## Layout

```
cmd/scopewall/          HTTP service entry point
internal/policy/        subject and resource matching, evaluation, delegation expansion, policy loading
internal/cache/         bounded LRU cache with TTL for decisions
internal/middleware/    HTTP middleware that wires identity, policy and cache together
policies/               example policy documents
docs/                   access model specification
```

## Build and test

```sh
go build ./...
go test ./...
```

## Run

```sh
go run ./cmd/scopewall -addr :8080 -policy policies/demo.json
```

The wrapped backend simply echoes what it received. Requests are authorized with three headers:

| Header | Meaning |
| --- | --- |
| `X-Scope-Service-Account` | identity of the calling workload |
| `X-Scope-Roles` | comma-separated role chain issued by the platform |
| `X-Scope-Role-Version` | opaque version of the caller's role chain |
| `X-Scope-Action` | action to authorize; derived from the method when absent |

Example:

```sh
curl -i http://localhost:8080/orgs/acme/projects/42 \
  -H 'X-Scope-Service-Account: svc-frontend' \
  -H 'X-Scope-Roles: platform-reader' \
  -H 'X-Scope-Role-Version: 7'
```

Allowed requests reach the backend and return `200`. Denied requests return `403` with a JSON
body carrying the decision reason. The decision source (`policy` or `cache`) is reported in the
`X-Scope-Decision` response header.
