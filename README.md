# hopguard — Edge Authorization Gateway

`hopguard` sits in front of a set of internal HTTP services. Clients call the
gateway; the gateway decides whether the call may proceed, then forwards it to
the upstream service that owns the path.

```
client ──▶ hopguard :8080 ──▶ upstream (orders | catalog | reports | public-site)
                  │
                  └─ reads keys/jwks.json, config/routes.json
```

## Layout

| Path | Purpose |
|---|---|
| `config/routes.json` | Listen address, token policy, upstream map, route table |
| `keys/jwks.json` | Public keys used to verify session tokens (test material) |
| `keys/test-private.jwk.json` | Matching private key for local token minting (test material) |
| `src/config.js` | Configuration loading and validation |
| `src/routes.js` | Route selection for an incoming request |
| `src/jwt.js` | Token verification and identity extraction |
| `src/scope.js` | Scope parsing and evaluation |
| `src/proxy.js` | HTTP server, authorization decision, upstream forwarding |
| `src/main.js` | Process entry point |
| `docs/gateway-authz.md` | **Normative** behaviour specification |
| `docs/incident-reports.md` | Penetration-test findings against an earlier gateway build |
| `test/` | `node --test` suite |
| `test-support/` | Shared fixtures for the test suite |

## Running

```sh
npm ci
npm test          # node --test
npm start         # listens on config.listen
```

The upstream addresses in `config/routes.json` are placeholders; nothing is
proxied unless those services are listening.

## Token shape

Session tokens are compact RS256 JWTs issued by the internal identity service.

```json
{
  "sub": "user-42",
  "roles": ["operator"],
  "scope": ["orders:read", "catalog:read"],
  "iss": "https://auth.internal.hopguard",
  "aud": "hopguard-edge",
  "iat": 1730000000,
  "nbf": 1729999995,
  "exp": 1730000600
}
```

`scope` may also be sent as a space-delimited string. `roles` is an array of
strings. Both are advisory to the gateway except for `scope`, which drives the
authorization decision.

## Route table

Routes are matched in declaration order; the first matching route wins. A route
declares the required scopes and the upstream that serves it. Requests that
match no route go to `defaultRoute`, or are answered with `404` when no default
is configured.

The normative rules for path matching, scope checks, token validation and
header handling live in [`gateway-authz.md`](./gateway-authz.md). That document
is the contract: if the implementation and the document disagree, the document
is right.

## Gateway responses

| Status | `code` | Meaning |
|---|---|---|
| 401 | `missing_token` | Protected route, no `Authorization: Bearer` value |
| 401 | `malformed` | Token is not a well-formed JWT |
| 401 | `signature` | Signature does not verify against the configured key set |
| 401 | `expired` | `exp` is outside the accepted window |
| 401 | `not_yet_valid` | `nbf` is in the future beyond the clock tolerance |
| 401 | `claims` | Required identity or policy claims are absent/invalid |
| 403 | `insufficient_scope` | Caller is authenticated but lacks a required scope |
| 404 | `not_found` | No route matched and no default route exists |
| 502 / 504 | `bad_gateway` / `gateway_timeout` | Upstream failure |

`403` responses list the absent scopes in `missing_scopes`. `401` responses
carry a `WWW-Authenticate: Bearer` challenge.

## Upstream contract

Upstream services trust only the headers written by the gateway:

- `X-Auth-Sub` — the authenticated subject (`sub`)
- `X-Auth-Roles` — comma-separated roles from the token

Any other identity-shaped header reaching an upstream is a gateway defect. See
`gateway-authz.md` §5.
