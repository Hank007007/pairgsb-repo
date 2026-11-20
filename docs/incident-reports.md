# Incident reports

Two incidents in the last quarter both involved the edge gateway accepting a
request it should have rejected, or forwarding something it should have
removed. Both were found by the downstream service, not by the gateway.

---

## INC-2214 — encoded path reached an administrative upstream

**Reported by:** accounts-admin on-call
**Severity:** high
**Impact:** a caller without an administrative scope reached the admin
upstream's user-deletion endpoint.

### What happened

The route table separates `/api/public` (no credential required) from
`/api/admin` (administrative scope required). An unauthenticated client
requested:

```
GET /api/public/%2e%2e/admin/users/8841
```

The gateway matched the request against `/api/public`, treated it as a public
route, and forwarded it. The upstream framework decoded the percent escapes and
resolved the dot segments before routing, so the request arrived at the admin
service as `DELETE`-equivalent work on a real account.

A second variant observed in the logs used doubled separators:

```
GET //api//public/../admin/users/8841
```

and a third used a trailing dot segment:

```
GET /api/public/./admin/users/
```

All three were matched as public. None of them produced an authorization
check.

### What the review concluded

The gateway and the upstream must agree on what the path *is* before either of
them makes a routing decision. The gateway decided on a string that the
upstream never saw. Route matching therefore has to run on a normalized path,
and normalization has to happen before the route table is consulted — not
after, and not twice.

The review also noted that the admin route was declared after the public route
in the table, and the table was consulted in declaration order. Even with a
normalized path, a request that matches two routes must be resolved
deterministically by specificity rather than by the order somebody happened to
write the file.

### Follow-ups

- Normalization rules written down as contract (§1 of `docs/route-auth.md`).
- Route selection by longest prefix (§2).
- Regression coverage for encoded, doubled-separator and trailing-dot forms.

---

## INC-2287 — upstream trusted a caller-supplied identity header

**Reported by:** reporting-service on-call
**Severity:** high
**Impact:** audit entries attributed to the wrong principal; one report was
exported under an administrator's name by a caller holding only `reports:read`.

### What happened

The reporting service builds audit records from `x-auth-user` and gates report
visibility on `x-auth-scopes`. It assumed those headers were gateway-owned.

A caller sent:

```
GET /reports/quarterly
Authorization: Bearer <valid token for sub=contractor-17, scope="reports:read">
X-Auth-User: dana.okafor
X-Auth-Scopes: reports:read admin:write
X-Forwarded-User: dana.okafor
```

The gateway forwarded the request headers as received, then attempted to add
its own identity headers. Because the header names were already present, the
gateway's values did not take effect. The reporting service saw the caller's
`x-auth-user` and `x-auth-scopes`, and the audit trail recorded the export as
dana.okafor's.

Separately, the review found that `Authorization` and `Cookie` were being
forwarded to internal services, which meant the caller's bearer token and
session cookie were readable by every upstream, including ones with weaker
access controls than the gateway.

### What the review concluded

An upstream must be able to treat `x-auth-*` as authoritative. That requires
two things: the gateway always overwrites those headers with values it derived
itself, and the gateway never forwards a caller-supplied copy of any header it
owns. Stripping is the safer primitive — the gateway should remove the
identity headers from the inbound request before it constructs the outbound
one, so that a route which sets no identity (a public route) still cannot leak
an identity through.

Header names are case-insensitive on the wire; the gateway had been comparing
them case-sensitively.

### Follow-ups

- Header ownership rules written down as contract (§6 of `docs/route-auth.md`).
- Caller credentials are never forwarded.
- Regression coverage for mixed-case identity headers on both authenticated and
  public routes.
