# Egress policy and transport

This document is normative. It defines destination allow-listing, redirect
handling, and retry behaviour for outbound webhook deliveries.

## 1. Scope

Every connection that leaves the platform toward a tenant endpoint is called a
**hop**. A delivery may involve several hops when the endpoint redirects. All
hops pass through the egress policy.

## 2. Destination matching

The integration carries an allow-list of **domains**. A destination is allowed
only when its host matches an allow-list entry by the rules below.

* Matching uses the **registrable host** of the destination: the hostname with
  any trailing dot removed, lowercased, and converted to its ASCII (punycode)
  form with the `idna` mapping of the first label-compatible form. A Unicode
  host and its punycode form are the same host.
* Ports and any userinfo are **not part of the domain**. A destination such as
  `https://user:pass@hooks.example:8443/` has host `hooks.example`, and it is
  allowed if and only if `hooks.example` matches an entry.
* An allow-list entry matches a destination when the destination host is
  **equal to** the entry, or when the destination host ends with `.` followed
  by the entry. Sentence: `example.com` matches `example.com` and
  `hooks.example.com`, and does not match `notexample.com` or
  `example.com.evil.io`. A bare `.` boundary is required, so an entry that is
  itself a suffix of a label (`ample.com`) does not match anything.
* An entry must be a registrable-looking host. An entry that contains a scheme,
  a path, a port, or userinfo is invalid configuration. An empty entry matches
  nothing.
* Matching is on the destination's **host**, not on the authority string and
  not on the path.

The allow-list applies to the initial destination and to every redirect target.
A redirect to a host not in the allow-list is refused; the delivery then ends
as a policy failure rather than being retried.

## 3. Schemes

Only `http` and `https` destinations are permitted. A redirect to another
scheme is refused. Plain `http` destinations are only permitted when the
integration enables insecure transport; otherwise a plain-`http` destination
(initial or redirected) is refused.

## 4. Redirects

A `3xx` response with a `Location` header is a redirect. The delivery follows
it only after the policy checks pass.

* The `Location` value is resolved against the current request-target. A
  `Location` that resolves to a different scheme, host, or port than the
  current hop is a **cross-origin** redirect. Comparing ports respects the
  default port of the scheme (`http` 80, `https` 443).
* On a cross-origin redirect the following are dropped before the next hop:
  `Authorization`, `Proxy-Authorization`, `Cookie`, and every signing header
  (`X-Outpost-Timestamp`, `X-Outpost-Signature`, `X-Outpost-Digest`). Any
  other tenant-supplied header is also dropped, because it was supplied for
  the original destination. Only headers the transport generates for the new
  hop are kept.
* On a same-origin redirect the request-target may change. The signature
  covers the request-target, so the delivery is re-signed for the new target
  using the same key. The body is unchanged.
* Redirect status codes followed are `301`, `302`, `303`, `307`, and `308`.
  `300` and `304` are not followed. `303` and any `301`/`302` answered to a
  non-`GET` request switch the method to `GET` and drop the body; the body
  digest is then recomputed for an empty body. `307` and `308` preserve the
  method and body.
* The maximum number of redirects per delivery (the **redirect budget**) is
  five. The budget is a property of the delivery, not of a single hop: it must
  be threaded through every hop, including retried hops. Exceeding it is a
  failure reported to the caller; the request is not sent a sixth time.
* `Location` must be a valid absolute or relative reference. A `Location`
  containing `\r`, `\n`, or NUL is invalid and the delivery fails.
* Every redirect hop is subject to §1–§3 before the connection is opened.

## 5. Retries

A `429` or `5xx` response is retryable. Retries are performed with exponential
backoff starting at the value given by `Retry-After` when present and valid
(seconds or an HTTP date), otherwise at a configured base delay, doubled per
attempt, and capped at a configured maximum.

* The retry budget is separate from the redirect budget.
* A retry re-sends the same request-target, headers, and body as the hop it
  retries; a retried hop does not consume a redirect.
* A retry is a new hop and passes the egress policy again.
* A `Retry-After` that exceeds the remaining retry budget is not honoured by
  sleeping; the delivery fails instead.

## 6. Failures

A policy failure, an invalid URL, an invalid header, a signature-coverage
failure, or an exhausted budget ends the delivery. The caller receives an
exception that names the failing stage. No partially modified request is sent.

## 7. Transport notes

* Connections are made with the standard library only.
* A `Host` header is always generated from the destination and may not be
  supplied by the tenant.
* `Content-Length` and `Transfer-Encoding` are generated by the transport and
  may not be supplied by the tenant.
* Response bodies are drained before the connection is reused or closed.