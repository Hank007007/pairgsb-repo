# Design notes

This package implements the outbound leg of the event platform: an event plus
a tenant "integration" becomes one HTTP request to the tenant's receiver, and
the platform follows redirects and retries transient failures.

The normative behaviour lives in four documents. This note states the contract
each pipeline stage must satisfy; it does not prescribe an implementation.

* `docs/urls.md` — base URL, path template, segment encoding, prefix containment
* `docs/query.md` — query encoding, merging, canonical ordering
* `docs/signature.md` — canonical string, header coverage, completeness
* `docs/egress.md` — allow-listing, redirects, retries

## Pipeline

```
event fields + integration          ->  rendered path        (urlbuild)
rendered path + template query       ->  query component      (queryset)
integration headers + event fields   ->  final header set     (headers)
final target + final headers + body  ->  signature            (sign)
signed request + policy              ->  responses, redirects (transport)
```

The key invariant is **fix the request shape before signing, then send exactly
what was signed**. Every stage before `transport` produces bytes that the
signer sees; `transport` may only change a request in ways `docs/egress.md`
§4 explicitly allows, and any such change is followed by re-signing (same
origin) or by dropping the signature (cross-origin).

## Stage contracts

### Path construction (`urlbuild`)

* A template variable is one whole segment, encoded with the segment-safe set
  in `docs/urls.md` §2.1 (`A-Za-z0-9-_~` only, percent uppercase, `.` encoded).
  The substitution is data, never syntax: no `/`, `?`, `#`, `%`, `.` or `..`
  produced by a value can change the path structure.
* The rendered path is appended to the base path prefix literally. No
  `urljoin`-style dot-segment removal may move the request outside the prefix.
  A rendered path that starts with `//`, contains `.`/`..`/empty segments, or
  otherwise changes the authority is invalid.
* The request-target handed to the signer includes the query component and
  nothing else (no scheme, no authority, no fragment).

### Query (`queryset`)

* Names and values are percent-encoded per `docs/query.md` §2, so `&`, `=`,
  `#`, `+`, and `%` in data cannot split, terminate, or reinterpret a pair.
* Template items and `params` merge per §3: groups by decoded name, template
  items first within a group, duplicates preserved, groups ordered by encoded
  name bytes.
* The query is final before signing. Nothing appends to it afterwards.

### Headers (`headers`)

* Static and dynamic values are validated: no CR, LF, or NUL; folded to the
  wire form; a forbidden header is reported by name.
* Header names are case-insensitive; one case-insensitive name yields one
  wire header. Tenant-supplied `Host`, `Content-Length`, `Transfer-Encoding`,
  and other transport-generated headers are rejected.
* The signed set is the tenant-supplied headers plus the generated signature
  headers, excluding the transport-generated ones.

### Signature (`sign`)

* The canonical string is exactly the composition in `docs/signature.md` §2,
  over the final request-target, the folded covered headers, the timestamp,
  and the body digest.
* If any transmitted, tenant-supplied header or any transmitted query pair is
  absent from the signed material, the delivery is refused (`signature.md` §5).

### Transport (`transport`)

* Every hop, including initial target, redirect target, and retry, passes the
  egress policy first.
* `3xx` with `Location` is followed only for the documented status set, only
  within the redirect budget, and with cross-origin credential/signature
  stripping plus re-signing for same-origin hops.
* `429`/`5xx` retries use the separate retry budget, `Retry-After` when
  valid, exponential backoff otherwise. Retrying a hop does not reset the
  redirect budget.

### Policy (`policy`)

* Matching is on the normalized destination host (`idna`, lower-case, no
  trailing dot), ignoring port and userinfo, with a label boundary. Entries
  are valid bare domains only: an entry like `ample.com` must not match
  `example.com`, `example.com.evil.io`, or `notexample.com`.

## What is observable

A local `http.server` recorder sees the actual request line, query string, and
headers, so every contract above can be checked end to end. In particular:

* the recorded target must equal the target the receiver recomputes the
  signature over;
* the recorded query pair set must equal the caller's `params` plus template
  items under the documented merge;
* the recorded header set must contain no injected header, no duplicate
  case-variant, and no tenant-supplied transport header;
* a redirect to a disallowed host, or beyond the budget, must fail rather
  than connect.

## Module map

| module | responsibility |
|---|---|
| `outpostcall/api.py` | `deliver()` orchestrates one delivery |
| `outpostcall/urlbuild.py` | base + template + fields → URL; request-target |
| `outpostcall/queryset.py` | params + template query → query component |
| `outpostcall/headers.py` | static/dynamic headers → header set |
| `outpostcall/sign.py` | canonical string, HMAC, signature headers |
| `outpostcall/transport.py` | send, redirects, retry/backoff |
| `outpostcall/policy.py` | allow-list and redirect decisions |
| `outpostcall/model.py` | `Integration`, `Event`, `DeliveryResult` |
| `outpostcall/errors.py` | exception hierarchy |
| `outpostcall/cli.py` | single-delivery command line |

## Non-goals

* No third-party HTTP client. Standard library only.
* No persistent connection pool; each hop uses a fresh connection.
* No retry of policy failures, URL errors, or signature-coverage failures.