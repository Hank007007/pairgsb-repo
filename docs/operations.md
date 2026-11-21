# Operations

## 1. Deployment

The resource server is a single Go binary started as `resource`. Configuration
comes from flags or environment variables:

| Setting | Meaning | Default |
|---|---|---|
| listen address | `host:port` to serve on | `127.0.0.1:8080` |
| introspection URL | authorization server endpoint | required |
| introspection client id / secret | credentials for the endpoint | required |
| introspection TTL | maximum positive cache lifetime | 10m |
| replay TTL | how long accepted proofs are remembered | 5m |

The process exposes `/healthz` (liveness, always `200` when serving) and
`/metrics` (counters for accepted and rejected requests, labelled by reason).
Protected routes sit behind the middleware.

## 2. Signing algorithms

The authorization server currently issues tokens bound to client keys of two
kinds:

- **ES256** — ECDSA over P-256 with SHA-256. This is the default for new
  clients.
- **RS256** — RSASSA-PKCS1-v1_5 over SHA-256, for clients on older stacks that
  cannot do EC.

Both are enabled at all times and the authorization server may switch a
client's key type at any moment. Keys are carried in the proof header, so no
out-of-band distribution is needed; the resource server must accept either
algorithm on any request and must not cache a "preferred algorithm" decision
across requests. Disabling an algorithm is a coordinated, announced change and
is not something the resource server does on its own.

## 3. Key rotation

The authorization server rotates its own token-signing keys, but that is
transparent: the resource server never verifies token signatures, only
introspects them. What rotates visibly is the *client's* key. A client may
generate a new key pair and request a new token bound to it at any time; the
old token remains valid until its `exp`. Both the old and the new key must
work for their respective tokens, which they do naturally because the key
comes from the proof header. There is therefore no key set to refresh and no
`kid` to resolve.

The consequence for the binding check is that it must be computed per request
from the presented key, for both EC and RSA keys, and must never fall back to
a previously seen fingerprint or to a cached binding keyed by anything other
than the token.

## 4. Clock skew

Hosts are synchronised with NTP. The 60-second window on `iat` is intended to
absorb measurement noise, not a misconfigured clock. Hosts drifting more than
30 seconds should be taken out of service. `exp` checks use the local clock and
are not widened.

## 5. Logging

Log the request id, route, method, status and the coarse rejection reason.
Never log the access token, the proof, the `jti`, or the client key. The
introspection client id may be logged.
