# Presign URL format

This document is the specification for links issued by urlseal. It is written
so that a second implementation in another language can interoperate with the
one in this repository: the canonical string below is a byte-level contract,
not an implementation detail.

## 1. Request

A presigned link represents one storage request:

| field     | source                    | notes                                   |
|-----------|---------------------------|-----------------------------------------|
| `method`  | caller / `X-Seal-Method`  | HTTP verb, uppercase ASCII              |
| `bucket`  | first path segment        | container name                          |
| `key`     | remaining path segments   | object key, may contain `/`             |
| `query`   | caller-supplied params    | protocol parameters excluded            |
| `expires` | `X-Seal-Expires`          | absolute unix time, seconds             |
| `tenant`  | `X-Seal-Tenant`           | key namespace                           |

## 2. Canonical string

The signed message is seven lines, each terminated by a single `\n`, with no
trailing newline after the last line:

```
urlseal-v1
<method>
<bucket>
<canonical key>
<canonical query>
<expires>
<tenant>
```

- Line 1 is the fixed format identifier `urlseal-v1`. It changes only if the
  canonical form itself changes.
- Line 2 is the HTTP method in uppercase.
- Line 3 is the bucket name verbatim.
- Line 4 is the canonical object key (§3).
- Line 5 is the canonical query string (§4), the empty string when the
  request has no caller-supplied query parameters.
- Line 6 is the expiry as a decimal unix timestamp in seconds, no padding.
- Line 7 is the tenant name verbatim.

The signature is `HMAC-SHA256(secret, canonical string)`, lowercase hex.

### 2.1 Worked example (cross-language test vector)

```
method   = GET
bucket   = assets
key      = reports/2024/q1.pdf
query    = a=1&b=2
expires  = 1767225600
tenant   = demo
secret   = demo-secret
```

Canonical string:

```
urlseal-v1
GET
assets
reports/2024/q1.pdf
a=1&b=2
1767225600
demo
```

The resulting `sig` is recorded in `test/vectors.test.js`; an implementation
that produces the same seven-line message and the same HMAC will match it.

## 3. Canonical key

The object key is taken from the URL path below the bucket.

1. Percent-decode each path segment into a key segment.
2. Reject the key if any segment is empty (including a leading, trailing or
   doubled `/`) — `a//b`, `/a`, and `a/b/` are not valid keys.
3. Reject the key if any segment is `.` or `..`.
4. Re-join the segments with a single `/`.

The canonical key is never obtained by normalizing the path: `bucket/a/../../b`
must not resolve to `b`. Such a key is rejected, not rewritten.

## 4. Canonical query

Only caller-supplied parameters participate. The protocol parameters
(`X-Seal-*` and `sig`) are removed before canonicalization.

1. Percent-decode each name and value.
2. Sort the pairs lexicographically by decoded name; pairs with the same name
   are ordered by decoded value. Sorting is byte-wise over UTF-8.
3. Re-encode name and value with RFC 3986 percent-encoding: unreserved
   characters (`A-Z a-z 0-9 - _ . ~`) stay literal, every other byte becomes
   `%XX` with uppercase hex digits. Spaces become `%20`, never `+`.
4. Join as `name=value` with `&`.

Repeated parameters are preserved as separate pairs — they are not collapsed.
Two different query strings must not canonicalize to the same line, and two
orderings of the same pairs must canonicalize identically.

## 5. Link layout

```
<host>/<bucket>/<key>?<caller query>&X-Seal-Alg=HS256&X-Seal-Method=GET
  &X-Seal-Expires=<unix>&X-Seal-Tenant=<tenant>&X-Seal-Version=<n>&sig=<hex>
```

`X-Seal-Version` names the key version used to sign; it is advisory and the
verifier determines the acceptable versions from its own keyring.
