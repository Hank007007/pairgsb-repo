# Token Cache — Design Notes

scopevault mediates every client-credentials exchange between internal
services.  A caller hands it an issuer, a client identifier and the set of
scopes it needs, and gets back an access token string.  Everything else —
exchanging, caching, refreshing, revoking — is the cache's business.

This document states the behaviour the cache is expected to have.  It is a
specification, not a design of the code: it says *what* must hold at the
boundary of `TokenCache`, not how to arrange the internals.

## §1 Public surface

§1.1 `TokenCache.get(issuer, client_id, scopes) -> str` is the only entry
point business code uses.  It returns an access token that is valid, belongs
to `issuer`, was minted for `client_id`, and carries at least the scopes that
were requested.

§1.2 `scopes` may be given either as a whitespace-separated string or as an
iterable of strings.  Both forms describe the same request when they denote
the same set; see `scope-policy.md`.

§1.3 `TokenCache.put(issuer, client_id, scopes, entry)` stores a token entry
explicitly.  It is used by maintenance tooling and by tests; callers that go
through `get` never need it.

§1.4 `TokenCache.revoke(issuer, client_id=None, scopes=None) -> int` removes
cached tokens.  `client_id` and `scopes` narrow the removal; when both are
omitted every token cached for that issuer is removed.  The return value is
the number of cache entries removed.

§1.5 Errors coming from the issuer surface as `TokenEndpointError`.  A failed
exchange never yields a token and never turns into a cached success.

## §2 Cache key and reuse

§2.1 The cache key is derived from the issuer, the client identifier and the
canonical scope set.  Two requests that differ only in the *order* or the
*repetition* of scopes share a key.

§2.2 Two requests that differ in the scope set do not share a key.  In
particular a token obtained for `read write` must not be served to a request
for `read` merely because the issuer and client match.

§2.3 A cached entry may be returned only when the set of scopes the issuer
actually granted for that token is a superset of the requested set.  A token
whose granted set is wider than requested is acceptable; a token whose
granted set is narrower — or different — is not.

§2.4 Isolation is per `(issuer, client_id)`.  A token cached for one client
identifier is never returned for another, even when both live at the same
issuer.

## §3 Lifetime

§3.1 A token is stored together with the instant at which it expires, derived
from the `expires_in` value reported by the issuer and the instant the
response was received.  Timestamps are compared in a single, unambiguous time
basis; the result must not depend on the machine's local time zone.

§3.2 Expiry is evaluated with a safety margin (clock skew).  A token whose
remaining lifetime is at or below the margin is treated as expired and is not
returned.  The margin applies uniformly: there is no request for which the
cache hands out a token that is at or past its expiry instant.

§3.3 An expired entry is never returned.  Encountering one causes a fresh
exchange, and the refreshed token replaces the stale entry.

§3.4 A cached entry is only ever returned while it is still within its
lifetime.  Nothing in the cache extends an entry's lifetime beyond what the
issuer granted.

## §4 Concurrent access

§4.1 When several threads request the same key and no usable entry is cached,
exactly one of them performs the exchange; the others receive that same
result.  This holds for the whole duration of the exchange, not merely for
part of it.

§4.2 A failed exchange is not a result.  It must not be recorded anywhere that
a later caller could mistake for a cached token.  After a failure, the next
request for that key performs a fresh exchange, so recovery needs no manual
intervention.

§4.3 Concurrent requests for *different* keys do not block one another.

## §5 Issuer responses

§5.1 A response is usable only when it carries a non-empty `access_token`.

§5.2 `token_type` must be `Bearer` (compared case-insensitively).  Any other
token type is an error; the token is not cached and not returned.

§5.3 `expires_in` must be present and must be a positive number of seconds.
A response without it is an error; the cache does not invent a lifetime.

§5.4 When the response carries a `scope` field, that field is the granted set
and is what §2.3 is evaluated against.  When it does not, the granted set is
the requested set.

## §6 Revocation

§6.1 `revoke` removes matching entries from the local cache.

§6.2 Revocation is also reported to the issuer that minted the token, through
that issuer's revocation endpoint.  A token that was revoked locally but never
reported is still live at the issuer.

§6.3 After a successful revoke, a subsequent request for the same issuer and
client does not reuse the revoked token.

§6.4 Revoking one key does not disturb unrelated entries.

## §7 Audit trail

§7.1 Cache hits, exchanges and revocations emit a structured log line naming
the issuer, the client identifier and the scopes involved.

§7.2 No log record — at any level, in any field, including exception text —
may contain the access token or any contiguous fragment of it.  Identifying a
particular exchange is done with a non-reversible digest or a generated
correlation id, never with a prefix, suffix or slice of the token.

§7.3 The audit trail is safe to ship to a central log store where access is
broader than access to the credentials themselves.
