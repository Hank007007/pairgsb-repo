# Rotation policy

This document is the specification for how certwhirl loads and replaces
certificate material. It describes observable behaviour, not an
implementation.

Terms:

- **leaf** — the end-entity certificate in a loaded pair.
- **trust pool** — the set of certificate authorities the deployment accepts.
- **identity** — the DNS name a certificate is expected to be valid for.
- **lead time** (`rotateBefore`) — how long before the leaf's `NotAfter` a
  replacement must begin.

## §1 Loading a pair

§1.1 A pair consists of a PEM certificate chain (leaf first, optional
intermediates after it) and a PEM private key. Loading fails if either file is
missing, unreadable, not valid PEM, or if the leaf and key do not match.

§1.2 The leaf must not be expired: loading fails when `NotAfter` is at or
before the load time.

§1.3 When a trust pool is configured, a pair is trusted only when its leaf
chains to one of the certificate authorities in that pool, using the presented
intermediates. Loading material whose issuer is not in the pool fails. A trust
pool that is configured but does not contain the issuing authority is not a
reason to skip the check.

§1.4 When one or more expected identities are configured, the leaf must be
valid for at least one of them according to its subject alternative names.
Loading a certificate issued for a different name fails. The common name alone
does not satisfy this requirement.

§1.5 Successful loading returns the parsed leaf, any intermediates, the raw
PEM material, and the load timestamp.

## §2 Deciding to rotate

§2.1 A replacement is due when the time remaining until the leaf's `NotAfter`
is strictly less than the configured lead time. At exactly the lead time the
certificate is still considered usable; one instant later it is due.

§2.2 Remaining time is measured with the clock configured on the rotator, so a
controllable clock can drive the decision in tests.

§2.3 When the remaining time is greater than or equal to the lead time, no
replacement is attempted: the loader must not be called and the published
configuration must not change.

§2.4 The default lead time is 72 hours. A leaf with 90 days of validity is
therefore due for replacement roughly 87 days after issuance, not at expiry.

§2.5 A leaf that is already expired, or a rotator with no current leaf, is
always due for replacement.

§2.6 The background loop evaluates §2.1 on every tick; the check interval does
not affect the decision itself, only how promptly it is noticed.

## §3 Replacement and publication

§3.1 The published `tls.Config` is swapped atomically. A reader never observes
a partially built configuration, and concurrent readers do not need a lock.

§3.2 Before publishing a replacement, the new material must be parsed into a
`tls.Certificate`. If parsing fails the replacement is rejected: the error is
returned to the caller (and reported to the error hook from the background
loop), and the previously published configuration stays in place.

§3.3 Publishing never clears the service's working configuration. There is no
state in which the rotator has no usable certificate because a replacement
attempt failed; the last known-good pair continues to serve handshakes.

§3.4 A successful replacement publishes the new certificate and records the
new pair as current.

§3.5 Failure to obtain new material from the loader is reported the same way
as §3.2: the error propagates, and the current certificate remains published.

§3.6 The initial load performed by `New` is different from a replacement: if it
fails there is nothing to fall back to, so construction fails.

## §4 Lifecycle

§4.1 `Start` launches the background loop and returns immediately. Calling it
again while running has no effect.

§4.2 `Stop` ends the loop and waits for it to return. It is safe on a rotator
that was never started.

§4.3 The published configuration, and the value returned by `TLSConfig`, is
never nil after `New` succeeds.
