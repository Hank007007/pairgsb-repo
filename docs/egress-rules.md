# Egress rules

This document is the normative contract between the platform security group, the
network group and the egressvault implementation. It states **what the proxy must
do**, not how it is built. Any behaviour that contradicts a rule here is a
security defect regardless of how convenient it is for callers.

Keywords: **MUST**, **MUST NOT**, **SHOULD** are used in the RFC 2119 sense.

---

## §1 Terms

| Term | Meaning |
|---|---|
| *caller* | The business process that sends plain HTTP requests to egressvault. |
| *upstream* | The real destination host, reached by egressvault over plain TLS. |
| *vault* | The local credential store read at start-up (see `vault.json`). |
| *rule* | One entry of the vault mapping a host pattern to one credential entry. |
| *injected headers* | The headers egressvault itself adds to an upstream request: the configured authentication header plus any extra headers declared by the credential. |
| *request host* | The `host` component of the absolute URI the caller asks egressvault to fetch, lower-cased, without a trailing dot, without port. |
| *hop* | One HTTP exchange between egressvault and an upstream. Redirects create additional hops. |

---

## §2 Host routing and credential selection

§2.1 A rule is `{ "host": "<name>", "credential": "<key id>", "headers": { ... } }`.
The `host` field is an exact DNS name, never a URL, never a pattern with wildcards.

§2.2 A request host matches a rule when, and only when, one of the following holds:

  a. the request host equals the rule host exactly; or
  b. the request host equals `<label>.<rule host>` for a non-empty `<label>` that
     does not itself contain a dot, i.e. the rule host is a **parent domain of the
     request host at a label boundary**.

A raw string suffix comparison is not a label-boundary comparison. With a rule for
`api.example.com`:

| request host | matches |
|---|---|
| `api.example.com` | yes (a) |
| `eu.api.example.com` | yes (b) |
| `a.b.api.example.com` | no — `<label>` contains a dot |
| `evil-api.example.com` | no |
| `xapi.example.com` | no |
| `api.example.com.evil.test` | no |
| `api.example.com.` (trailing dot) | yes — the trailing dot is removed first |
| `API.EXAMPLE.COM` | yes — comparison is case-insensitive |

§2.3 When several rules match, the longest rule host wins. If two distinct rules
declare the same host the vault is invalid and the proxy MUST refuse to start.

§2.4 When no rule matches, no credential is attached and the request is forwarded
unauthenticated. The audit record for that hop carries no key id.

§2.5 The matching function is pure: it depends only on the request host and the
rule set. It MUST NOT consult environment variables, caller supplied headers, DNS
resolution results, IP literals or the path.

---

## §3 Credential injection

§3.1 The credential is looked up in the vault by key id for every hop, never cached
across hops and never derived from the caller's request.

§3.2 The caller MUST NOT be able to influence which credential is used. If the
caller supplies a header with the same name as the configured authentication
header, egressvault MUST discard the caller's value and write the vault value. If
the caller supplies any header listed in the credential's `headers` map, the same
applies. Callers cannot opt out of injection for a host that has a rule.

§3.3 The default authentication header is `Authorization` with the credential's
`scheme` (`Bearer`, `Basic`, …) and secret. A credential may instead declare
`header` to use a custom header name; the value is then the secret alone.

§3.4 Credential material MUST NOT leave the proxy process except on the wire to the
matched upstream. It MUST NOT be written to logs, audit records, error messages,
stack traces, crash dumps, or returned to the caller in any form. Upstream
responses are relayed unchanged; the proxy does not inspect or rewrite headers on
the way back, but it also never echoes the credential it injected.

§3.5 Injection happens immediately before the request is written to the upstream
socket, after redirect handling has decided the final hop target (§4).

---

## §4 Redirects

§4.1 egressvault follows at most `EGRESS_MAX_REDIRECTS` hops, default 5. Exceeding
the limit is an error (`ELOOP`), returned to the caller as `502`.

§4.2 Only `301`, `302`, `303`, `307` and `308` are followed. A `300`, `304` or any
other status is returned to the caller untouched.

§4.3 **Header handling on follow.** Let `from` be the host of the hop that produced
the redirect and `to` the host of the `Location` target, both normalised as in §1.

  a. If `to` is the same host as `from`, injected headers are preserved and the
     request keeps its method and body (`303` still downgrades to `GET`, see §4.4).
  b. If `to` differs from `from` in any way — different host, different port,
     different scheme, or a downgrade from `https` to `http` — **all injected
     headers from the previous hop MUST be stripped before the next hop is built**.
     The new hop is then evaluated against §2 independently and receives *only*
     the credential (if any) that its own host matches.
  c. Caller-supplied headers are forwarded across a redirect only when the target
     is the same host; across hosts they are dropped, because a caller header may
     itself carry application secrets that §3.4 forbids leaking to a third party.

§4.4 Status semantics: `303` always turns the follow-up into `GET` with no body and
no `Content-Type`/`Content-Length`. `301` and `302` keep the method for `GET`/`HEAD`
and are otherwise downgraded to `GET` without a body. `307` and `308` preserve
method and body byte-for-byte.

§4.5 A `Location` value is resolved relative to the redirecting URL (RFC 3986
§5.2). Scheme-relative (`//host/path`), absolute-path, relative-path and fragment
forms are all valid. A `Location` with a scheme other than `http` or `https` is an
error (`EBADREDIRECT`, `502` to the caller).

§4.6 Redirect targets are subject to §2 and §5 exactly like first hops. The proxy
never follows a redirect straight to a raw socket without re-running routing and
TLS validation.

§4.7 `Set-Cookie` from a redirect response is not forwarded to subsequent hops.
The caller receives the redirect chain's final response only.

---

## §5 Upstream TLS

§5.1 Every upstream connection is TLS with certificate verification enabled.
`rejectUnauthorized` is always `true` for upstream sockets. No environment
variable, caller header, vault field, config file or command-line flag may disable
or weaken it, and there is no "insecure" mode.

§5.2 The trust store is the platform CA bundle (`NODE_EXTRA_CA_CERTS` and the
system store). A separate `EGRESS_CA_FILE` may add CAs for the egress fleet; it
never removes the system store.

§5.3 The peer certificate is validated against the host name of the hop
(SNI and `servername` set to that host). IP-literal upstreams are permitted only
when a rule matches the literal and the certificate contains a matching `iPAddress`
SAN.

§5.4 A verification failure aborts the hop. The caller receives `502` with code
`EUPSTREAMTLS`; the underlying OpenSSL reason string is logged at debug level only
and never includes any part of the request or credential.

§5.5 Client certificates, when a credential declares one, are selected by the key
id of the credential chosen in §2, never by SNI sniffing of a shared socket.
`passphrase` material follows §3.4.

§5.6 TLS sessions may be reused within a host only. A pooled connection is never
reused for a different host or a different credential.

---

## §6 Audit log

§6.1 One JSON object per line (JSONL), appended, flushed per record. A record is
written for every hop, including hops that fail.

§6.2 The **only** fields a record may contain are:

| field | type | meaning |
|---|---|---|
| `ts` | RFC 3339 string | when the hop started |
| `host` | string | request host of the hop, as in §1 |
| `path` | string | path plus query, with the query string redacted to its first key name (`?a=1&b=2` → `?a=…`) |
| `status` | integer or null | upstream status, `null` if no response |
| `durationMs` | number | wall time of the hop |
| `keyId` | string or null | key id of the credential used, `null` if none |
| `error` | string or null | error code, never a message, never a stack |

§6.3 A record MUST NOT contain: header names or values (of either direction), the
request or response body, cookies, the credential secret, the credential's scheme,
`Location` URLs, caller identity beyond what the fields above carry, or the TLS
peer certificate.

§6.4 Redaction is by construction: the record is built from named fields. Serialising
a request object and deleting sensitive keys afterwards is not acceptable — one
forgotten key leaks a credential.

§6.5 The audit file MUST be created with owner-only permissions where the platform
supports it. Rotation and retention are the operator's concern; the proxy only
appends.

---

## §7 Proxy environment variables

§7.1 The proxy reads, in addition to `EGRESS_VAULT_FILE`, `EGRESS_AUDIT_FILE`,
`EGRESS_MAX_REDIRECTS` and `EGRESS_CA_FILE`:

| variable | role |
|---|---|
| `HTTP_PROXY`, `http_proxy`, `ALL_PROXY`, `all_proxy` | upstream proxy for `http://` hops |
| `HTTPS_PROXY`, `https_proxy`, `ALL_PROXY`, `all_proxy` | upstream proxy for `https://` hops |
| `NO_PROXY`, `no_proxy` | hosts that bypass the upstream proxy |

§7.2 For each scheme, an upper-case variable wins over its lower-case twin. If only
the lower-case form is set it MUST still be honoured; a deployment that exports
`https_proxy` must not silently egress directly.

§7.3 `NO_PROXY` matches on the request host with the same label-boundary rules as
§2.2. `*.example.com` and a leading-dot `.example.com` mean "the domain and all
its subdomains"; a bare `example.com` means exactly that host. A port suffix
(`host:8080`) restricts the match to that port. `*` disables the upstream proxy
entirely. Values are comma or whitespace separated, case-insensitive.

§7.4 **Precedence is fixed: `NO_PROXY` is evaluated first.** If the request host
matches `NO_PROXY`, the hop goes directly to the upstream and the configured proxy
variables are ignored for that hop. Only when `NO_PROXY` does not match is the
scheme's proxy variable consulted. A host listed in `NO_PROXY` MUST NOT be dialled
through the upstream proxy even when `HTTP_PROXY` and `HTTPS_PROXY` are set.

§7.5 `NO_PROXY` is a *network routing* rule, not an authentication rule. A host that
is in `NO_PROXY` still gets its credential from §2 when it is a first hop; a host
reached through the upstream proxy is likewise routed by §2. Proxy routing never
changes which credential a request carries, and it never sends a request carrying
an injected header to a host that §2 did not select.

§7.6 Proxied hops use the standard absolute-form request target for `http://` and
`CONNECT` for `https://`. An upstream proxy is itself addressed with `http://`
unless the variable value says otherwise, and a `Proxy-Authorization` header from
the same vault rules as §3 applies to the proxy hop only — it is never forwarded to
the upstream.

---

## §8 Errors

§8.1 Error responses to the caller use a small fixed set of codes: `EBADREQUEST`,
`EUNSUPPORTEDSCHEME`, `ENOROUTE`, `EUPSTREAMTLS`, `EUPSTREAM`, `ELOOP`,
`EBADREDIRECT`, `ETIMEOUT`. The body is `{"error":{"code":"…"}}`; no upstream or
credential detail is echoed.

§8.2 The proxy listens on loopback only unless `EGRESS_BIND` names another address.
A caller that is not on an allow-listed network is refused before routing runs.
