# Incident reports — edge gateway

Findings reported against the edge gateway by the internal security team.
Each entry records what was observed during a penetration test, kept as
background for whoever works on the gateway next. The behaviour required of
the service is defined in [`gateway-authz.md`](./gateway-authz.md); these
reports describe symptoms, not remedies.

---

## IR-2023-014 — Public prefix reachable from a protected path

**Severity:** critical
**Reported:** 2023-04-11

A tester requested `/public/../admin/export` and received the export body from
the reporting service, with no caller identity headers on the request.
Requesting `/%2e%2e/admin` behaved the same way, and
`/public/%2e%2e/%2e%2e/admin/export` with it.

The reporting service logs the path it actually serves: `/admin/export`,
`/admin`, and `/admin/export` — not the targets the tester sent.

**Observed:** requests whose raw target began with `/public` were routed to the
public route regardless of where the path resolved. A related probe,
`/publicity/x`, was also served by the public route.

---

## IR-2023-021 — Read-only grant accepted for a read-write endpoint

**Severity:** high
**Reported:** 2023-05-02

A contractor account provisioned with `orders:readonly` was able to call the
orders listing endpoint, which declares `orders:read`. The same account was
able to call `admin:export`-protected endpoints while holding only
`admin:export:audit`.

**Observed:** a granted scope was treated as satisfying any required scope it
began with.

---

## IR-2023-027 — Forged identity headers accepted by upstream

**Severity:** critical
**Reported:** 2023-05-30

Two of the internal services trusted `X-Auth-User` and `X-Auth-Roles`
verbatim. A tester sent those headers to a protected endpoint alongside a
valid token for a low-privilege subject. The upstream saw both the token's
identity and the forged values; the service's authorization layer read the
forged role and permitted the call.

The same technique worked against a public endpoint, where no token was
presented at all: the upstream received `X-Forwarded-User: administrator`.

**Observed:** headers supplied by the caller reached the upstream. Only one of
the several identity header names in use across the fleet was being removed.

---

## IR-2023-033 — Token accepted long after expiry

**Severity:** high
**Reported:** 2023-06-18

A captured token continued to be accepted for roughly an hour past its `exp`
value. Testers also replayed a token whose `nbf` was two minutes in the
future; it was accepted immediately.

**Observed:** the acceptance window did not match the documented 30-second
clock tolerance.

---

## IR-2023-041 — Algorithm and key substitution

**Severity:** critical
**Reported:** 2023-07-09

Testers minted tokens with `alg: RS512` using the leaked test signing key and
were accepted. A token claiming a `kid` that does not exist in the published
JWKS was also accepted, as was a token whose `iss` named a different issuer
entirely and whose `aud` was `some-other-service`.

**Observed:** the verification step accepted any algorithm the configured key
could produce, fell back to an arbitrary key when `kid` was unknown, and did
not evaluate `iss` or `aud`.

---

## IR-2023-052 — Header value ambiguity at one upstream

**Severity:** medium
**Reported:** 2023-08-14

Following IR-2023-027, an upstream began receiving two differently-cased
copies of `X-Auth-Roles` on the same request. Depending on the HTTP client
library, the service saw either the first value, the last value, or the two
joined with `, `. One library concatenated them in a way that made a
low-privilege role appear alongside an administrative one.

**Observed:** removal and rewriting must agree on case, and the upstream must
receive a single value for each identity header.
