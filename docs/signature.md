# Request signing

This document is normative. It defines the canonical string, the HMAC, and the
headers a receiving endpoint uses to verify a delivery.

## 1. Key and algorithm

The signing key is a byte string configured on the integration. The MAC is
`HMAC-SHA256` over the UTF-8 bytes of the canonical string. The result is
lowercase hexadecimal.

## 2. Canonical string

The canonical string is built from exactly these lines, each terminated by a
single `\n` (LF), with no trailing blank line:

```
<method>\n
<request-target>\n
<signed-header-lines>\n
<timestamp>\n
<body-digest>\n
```

More precisely, the canonical string is the concatenation, in order, of:

1. the uppercased HTTP method;
2. the **request-target** exactly as it will appear on the wire: the path
   including the base prefix and the fully encoded template segments, followed
   by `?` and the fully encoded, canonically ordered query component when a
   query is present, and nothing after it (no scheme, no authority, no
   fragment);
3. the signed headers, one `name:value` per line, where `name` is lowercase
   and `value` is the header value after invisible folding (§3). Lines are
   sorted by lowercase name, then joined by `\n`;
4. the timestamp, decimal seconds since the Unix epoch;
5. the lowercase hex SHA-256 of the exact request body bytes;
6. each of the above lines is terminated with `\n`; an empty signed-header
   block still contributes one empty line.

The canonical string does not contain the scheme or authority. A signature
therefore covers a request-target, not an origin, and the transport rules in
`docs/egress.md` §4 govern whether a signature may be reused elsewhere.

## 3. Header coverage

Only headers named in the integration's signing configuration are covered.
The signer always adds the following to the covered set:

* the timestamp header,
* the body-digest header,
* every fixed header and every dynamic header the integration declares.

The following headers are never covered, even if listed:

* `host`, `content-length`, `transfer-encoding`, `connection`, `te`,
  `trailer`, `upgrade`,
* any header whose name is not a valid HTTP field name,
* any header that the transport itself generates after signing.

A header value may not contain `\r`, `\n`, or NUL. Header values are folded by
trimming leading and trailing spaces and tabs and collapsing internal runs of
spaces and tabs to a single space before the canonical string is computed; the
wire value is the same folded value.

## 4. Encoding used by the canonical request-target

The request-target reuses the exact encoding of `docs/urls.md` §2.1 for path
segments and of `docs/query.md` §2 for names and values. Percent escapes are
uppercase. Nothing is double-encoded: the path stored in the canonical string
is the rendered path, not a re-encoding of an already encoded path.

## 5. Completeness rule

A delivery may only be signed if the signer observes the final request-target
and the final header set. If any of the following is true, the request must
not be sent:

* a header that will be transmitted is not covered by the signature and was
  supplied by the tenant or by an event field (transport-generated headers
  such as `Host` and `Content-Length` are exempt);
* a query parameter that will be transmitted is absent from the signed query
  component;
* the request-target on the wire would differ in any byte, including escape
  case or pair ordering, from the signed request-target;
* the body is modified after the digest is computed.

In short: the signer signs the final request, and the transport sends the
signed request. A request whose target or covered headers were altered after
signing is rejected rather than sent.

## 6. Headers transmitted

The delivery carries the signature as follows:

* `X-Outpost-Timestamp` — decimal seconds.
* `X-Outpost-Signature` — `v1=<hex>`.
* `X-Outpost-Digest` — `sha-256=<hex>`.

These three are transport-generated and are not themselves covered.

A `Signature-Input` style structured field is not used. The canonical string
above is the only definition.