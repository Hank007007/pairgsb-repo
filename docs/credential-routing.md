# Credential routing rules

This document is normative. It defines what a correct credmux deployment must do
when injecting credentials into outbound requests. It deliberately describes
observable behavior only; it does not prescribe an implementation.

The key words MUST, MUST NOT and SHOULD are used in their usual sense.

---

## §1 Secrets bundle

A secrets bundle is a JSON document with the following top-level shape:

```json
{
  "version": 1,
  "secrets": [
    {
      "id": "partner-api",
      "host": "api.partner.com",
      "scheme": "bearer",
      "value": "tok_live_123",
      "not_before": "2026-01-01T00:00:00Z",
      "expires_at": "2026-12-31T23:59:59Z"
    }
  ]
}
```

1.1 `id` MUST be unique within a bundle. Duplicate ids make the bundle invalid.

1.2 `host` is the only routing key. It MAY contain a port (`api.example.com:8443`)
    or be a bare hostname (`api.example.com`). It MUST NOT contain a scheme,
    path, query, userinfo or wildcard.

1.3 `scheme` selects how the value is presented on the wire. The supported
    values are `bearer` and `api-key`.

1.4 `not_before` and `expires_at` are optional RFC 3339 timestamps. When absent,
    the corresponding bound is open. When present they MUST be parsed and
    retained as absolute instants.

1.5 `value` is the secret. It MUST never appear in any audit record, error
    message, stack trace or log line in clear text.

1.6 A bundle is invalid if the JSON cannot be parsed, if a required field is
    missing or the wrong type, if `scheme` is unsupported, or if a timestamp is
    present but unparseable. An invalid load MUST leave the previously loaded
    bundle in effect.

---

## §2 Host matching

2.1 A credential entry applies to a request **only if the entry's `host` and the
    request's effective host are the same host**, where "same host" means:

    - hostnames compare case-insensitively;
    - an entry without a port applies to the default port for the request's
      scheme (80 for `http`, 443 for `https`) and to no other port;
    - an entry with a port applies only when the request's effective port is
      exactly that port;
    - a trailing root dot in a hostname is insignificant (`api.example.com.`
      is the same host as `api.example.com`);
    - IPv6 literals compare in their normalized `[addr]` form.

2.2 Substring, prefix, suffix or "contains" matching is NOT acceptable. In
    particular:

    - `api.partner.com.attacker.example` MUST NOT match an entry for
      `api.partner.com`;
    - `evil-api.partner.com` MUST NOT match an entry for `api.partner.com`;
    - `api.partner.com:8080` MUST NOT match a portless entry for
      `api.partner.com` when the request is made over `https`;
    - `api.partner.com.` MAY match an entry for `api.partner.com`.

2.3 If several entries could match, the most specific one wins (entry with an
    explicit port beats a portless entry). If the bundle has more than one
    equally specific entry for the same host and scheme, the bundle is
    ambiguous and the loader MUST reject it.

2.4 If no entry matches, the request proceeds **without any credential**. It is
    never correct to fall back to another host's credential, to a "closest"
    host, or to the previously used credential.

---

## §3 Temporal validity

3.1 An entry MAY only be selected while
    `not_before <= now < expires_at`, evaluating each bound only when present.
    "Now" is the instant the request (or retry attempt) is actually sent, not
    the instant the bundle was loaded or the request was queued.

3.2 An entry whose `not_before` lies in the future MUST be treated exactly like
    an entry that does not exist for that host: no header is injected.

3.3 An entry at or past `expires_at` MUST be treated exactly like an entry that
    does not exist for that host.

3.4 If the only matching entry for a host is temporally invalid, no credential
    is sent. The relay MUST NOT substitute a different entry for a different
    host, and MUST NOT send the stale value anyway.

3.5 Evaluation MUST use an injectable clock so that time-dependent behavior is
    testable without sleeping.

---

## §4 Injection

4.1 For scheme `bearer` the request MUST carry
    `Authorization: Bearer <value>`.

4.2 For scheme `api-key` the request MUST carry `X-Api-Key: <value>`.

4.3 Injection MUST replace any caller-supplied value for the same header
    (case-insensitively) rather than emitting two headers with different
    casing.

4.4 Headers unrelated to credential injection MUST be preserved.

4.5 The relay MUST NOT inject a credential into a URL that is not routed to a
    matching entry, including URLs reached by redirect or retry.

---

## §5 Redirects

5.1 A redirect is followed only when the response carries a `Location` header
    and the status is one of 301, 302, 303, 307, 308.

5.2 On 303, and on 301/302 where the original method was `POST`, the follow-up
    request MUST use `GET` and MUST NOT carry a body. On 307/308 the method and
    body are preserved.

5.3 The follow-up request MUST be routed from scratch: the effective host of
    the new URL is computed, credential selection (§2, §3) is repeated for that
    host, and headers are rebuilt. **Headers from the previous hop MUST NOT be
    carried over wholesale.**

5.4 If the new host has a matching, temporally valid entry, that host's
    credential is injected (§4). If it does not, the follow-up carries no
    credential header at all — in particular the source host's credential MUST
    NOT appear in the follow-up request.

5.5 Non-credential headers explicitly supplied by the caller (e.g.
    `Accept`, `User-Agent`, `Content-Type`) SHOULD be preserved across hops.
    Caller-supplied credential headers MUST be treated as hop-local and MUST NOT
    follow a cross-host redirect.

5.6 Relative `Location` values resolve against the current URL. A redirect
    loop exceeding `maxRedirects` MUST fail with a `TooManyRedirectsError` and
    MUST be reported to the audit log without including any secret value.

5.7 A redirect from `https` to `http` MUST be refused by default. An opt-in
    `allowInsecureRedirects` flag may permit it, but even then §5.3 and §5.4
    still apply.

---

## §6 Retries and hot reload

6.1 Retries apply to transport errors, status 429, and statuses 500, 502, 503
    and 504, up to `maxAttempts` (default 3) with exponential backoff and
    jitter, bounded by `baseDelayMs` and `maxDelayMs`.

6.2 A retry is a **new outbound request**. On every attempt the relay MUST
    re-derive the request headers from the *currently loaded* bundle: re-run
    host matching (§2) and temporal validity (§3) against the attempt's send
    time.

6.3 A queued retry MUST NOT capture and replay the credential value, the
    credential id alone, or a header snapshot taken when it was scheduled.
    After a bundle reload the queued attempt uses whatever entry is valid for
    that host at send time.

6.4 If, at the moment a queued retry fires, the entry that originally supplied
    the credential is no longer present in the bundle (or is no longer valid by
    §3), the relay MUST NOT send that credential. If another entry matches the
    host, that entry is used; otherwise the retry is sent without a credential
    or is abandoned with an auditable error, but in no case does the old value
    or old id get transmitted.

6.5 Hot reload MUST be atomic: a request that has already begun selecting an
    entry finishes with a consistent view; the next attempt sees the new
    revision. No request may observe a half-updated bundle.

6.6 Only one reload may be in flight at a time. A failed reload leaves the
    previous revision active (§1.6).

---

## §7 Audit log

7.1 Every audit record MUST be safe to ship to a third-party log sink. No
    record may contain a secret in clear text.

7.2 Header redaction MUST be case-insensitive. `Authorization`,
    `authorization`, `AUTHORIZATION`, `X-Api-Key`, `x-api-key` and any other
    casing MUST all have their values replaced with a fixed placeholder
    (`[REDACTED]`).

7.3 Every value present in the active bundle MUST be redacted wherever it
    appears, not only in the two conventional header names. This includes:

    - values in arbitrary headers (custom auth headers, cookies, proxies);
    - values appearing in the URL query string (e.g. `?api_key=...`,
      `?token=...`, `?access_token=...`);
    - values appearing in a redirect `Location`;
    - values embedded in error messages.

7.4 URLs in audit records MUST be normalized for logging: known secret-bearing
    query parameters are replaced with the placeholder, and any bundle value
    found as a substring of the URL is replaced as well. Path segments that
    happen to equal a secret value MUST also be redacted.

7.5 Redaction MUST be applied after the record is assembled but before it is
    stored or emitted, so that no intermediate object is ever logged.

7.6 Redaction MUST NOT be defeated by percent-encoding or by mixed casing of
    the parameter name. When a value can be decoded safely, both the encoded
    and decoded forms SHOULD be redacted.

7.7 The audit record SHOULD include the credential `id` (never the value), the
    request method, the redacted URL, the redacted headers, the attempt number,
    the outcome status, and a monotonically increasing sequence number.

7.8 Audit writes MUST NOT throw into the caller's request path; a failing
    audit sink is reported on stderr and the request continues.

---

## §8 Relay surface

8.1 Business code calls `relay.request(url, opts)`. `opts` may include
    `method`, `headers`, `body`, `timeoutMs`, `signal` and `maxRedirects`.

8.2 `relay.request` resolves with `{ status, headers, body, url, attempts,
    redirects, redactedHeaders }` for any final non-redirect response,
    including 4xx/5xx that are not retryable. It rejects only on transport
    failure after retries are exhausted, on redirect loops, or on invalid
    options.

8.3 All injected credential material MUST be confined to the relay's outbound
    request. The resolved value MUST NOT contain an injected credential header
    in clear text; `redactedHeaders` MUST be redacted per §7.

8.4 `relay.close()` stops the reload timer and drains or rejects queued
    retries; pending timers MUST NOT keep the process alive.

---

## §9 Diagnostics

9.1 `relay.diagnostics()` returns the active bundle `revision`, the number of
    entries, the queued retry count, and counters for requests, retries,
    redirects and redactions. It MUST NOT expose secret values.

9.2 Errors surfaced to callers MUST carry a stable `code`
    (`BUNDLE_INVALID`, `TOO_MANY_REDIRECTS`, `INSECURE_REDIRECT`,
    `TRANSPORT_ERROR`, `ABORTED`) and MUST NOT embed credential values.
