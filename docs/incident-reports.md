# Incident Reports — Sealed Cookie Platform

Two post-incident reviews are recorded here. They are published so that
service owners understand which guarantees the envelope must provide. Root
cause analysis in these reviews is written at the level of observable
behaviour; the corrective requirements are normative for this library and are
reflected in `docs/cookie-envelope.md`.

---

## IR-2025-014 — Cookie accepted by a service it was not issued for

**Severity:** High
**Status:** Closed
**Affected surface:** internal read path, service-to-service trust boundary

### Summary

A cookie issued for the billing console was presented to the profile service
and accepted. The profile service returned a subject identity that had never
signed in to it, and its response included the console's permission scopes.
The cookie was a legitimate, correctly signed cookie. It had been issued to a
different audience minutes earlier.

### Timeline (UTC)

- 09:41 — Operator signs in to the billing console. Login issues a cookie.
- 09:52 — The same cookie value is replayed against the profile service, from
  a request path that the operator did not initiate. The profile service
  accepts it and serves the operator's console scopes.
- 09:58 — An anomaly rule on scope usage in the profile service fires.
- 10:20 — Services holding the shared key ring are identified. The replay
  window is found to have been limited by the cookie's short lifetime only.
- 11:05 — Affected services are drained and the incident is contained.

### Impact

One account's console-scoped claims were readable by a service outside its
intended audience for approximately six minutes of wall-clock exposure. No
write action was taken with the replayed identity. The blast radius was
bounded by incident response, not by the envelope.

### Observations from the review

- Signatures prove *who issued* a cookie, not *who may read* it. Every
  service in the trust domain held the key ring and could verify a cookie that
  was never addressed to it.
- The cookie carried an audience claim, but the read path did not consult it.
  A valid MAC was treated as sufficient authorisation.
- The same cookie string was accepted indefinitely within its lifetime by any
  reader, so replay across services was indistinguishable from a normal read.
- Logs keyed by subject did not reveal the cross-service reuse until scope
  anomalies were correlated manually.

### Corrective requirements

- Readers must reject a cookie whose audience does not name the calling
  service. Key possession alone must not authorise a read.
- Cookies with different trust boundaries must carry disjoint audiences; the
  issuer is responsible for addressing each cookie to the services that need
  it.
- The audience check must happen for every read, alongside the validity
  window, and must not be bypassable by a caller-supplied option.

---

## IR-2025-031 — Two different cookie strings for the same identity

**Severity:** Medium
**Status:** Closed
**Affected surface:** envelope parsing, logs, replay cache

### Summary

During a cache-key investigation, engineers found multiple distinct cookie
strings that decoded to the same header, body, and claims. In one case, a
cookie copied from a log with its final character altered still authenticated
and produced the same subject. In another, a client library appended extra
segments to the serialised cookie; readers ignored them and returned normal
claims.

### Timeline (UTC)

- 14:03 — Cache hit ratios for subject lookups diverge from expectation.
- 14:30 — A sampling tool enumerates cookie strings for one test identity.
  Several spellings of the same claims are collected.
- 15:10 — The alternative spellings are traced to permissive decoding in the
  read path: characters outside the intended alphabet were skipped, and
  surplus segments were discarded.
- 16:00 — Log and replay-cache keys are rebuilt from the affected window.

### Impact

- Replay caches and rate limiters keyed by the cookie string saw one
  credential as several, so limits under-counted.
- Log correlation and incident forensics were unreliable: two strings that
  looked different were the same credential.
- No authentication bypass was demonstrated from the alternative spellings
  alone, but the review classified the finding as security-relevant because
  it defeats string-keyed controls that the platform relies on.

### Observations from the review

- The reader accepted segment strings that were not canonical encodings and
  did not reject surplus segments. The parser's leniency created aliases.
- Authentication covered only part of the envelope, so some changes to the
  serialised form survived verification.
- Nothing in the contract stated that a credential has exactly one
  serialisation, so client libraries felt free to decorate the string.

### Corrective requirements

- The envelope must have one canonical serialisation for a given set of
  segments. Readers accept exactly four segments, exact base64url, no
  padding, and no alternative characters.
- Any non-canonical spelling is rejected as malformed rather than repaired.
- Authentication must cover the serialised form that is actually read, so
  that no part of the string can be altered independently of the tag.
- Downstream controls may safely key on the cookie string because string
  identity and credential identity coincide.

---

## Status of corrective work

The requirements above are captured normatively in `docs/cookie-envelope.md`
(§2 canonical segmentation, §4 message authentication, §7.4 audience). This
repository is the reference codec for those requirements.
