# sessforge session policy

This document is the normative specification for sessforge.  It describes the
behaviour a deployment is expected to provide.  It intentionally contains no
implementation guidance: module and function layout is free as long as the
observable behaviour below holds.

The key words MUST, MUST NOT, SHOULD and MAY are used in their RFC 2119 sense.

## §1 Scope

sessforge manages state that exists after (and around) a successful login:

- issuing the session cookie,
- keeping the authoritative server-side session record,
- deciding whether a presented session is still usable,
- logging out, revoking individual sessions and revoking all sessions of a user,
- the "remember me" credential.

A session is an opaque identifier (`sid`) that the client stores in a cookie.
Everything that determines validity lives on the server.

## §2 Session identifiers

1. A `sid` MUST be generated from a cryptographically secure random source.
2. A `sid` MUST contain at least 128 bits of entropy.
3. A `sid` MUST NOT be derived from user data, timestamps, counters, or any
   value that an outside party could predict or influence.
4. A `sid` MUST be unique among stored sessions.  Two live sessions MUST NOT
   share an identifier.

## §3 Issuance and login

1. An unauthenticated visitor MAY be given an anonymous session so that
   pre-login state (for example a CSRF token or a shopping cart) has somewhere
   to live.
2. On a successful login the service MUST issue a brand-new `sid`.  The
   identifier the browser presented before login MUST NOT become the
   authenticated session.
3. After the new `sid` is issued, the pre-login anonymous session MUST no
   longer be resolvable.  Presenting the old value MUST NOT yield either the
   anonymous record or an authenticated one.
4. The same rotation rule applies whenever a session changes privilege level
   (for example an anonymous checkout session upgraded after login, or a
   session that gains administrative rights).
5. The new authenticated session record MUST carry the identity, creation
   time, current epoch and client metadata (user agent, address) of the
   request that performed the login.

## §4 Session lifetime

Two independent deadlines govern every session.

1. **Idle timeout.** A session that has not been used for 30 minutes MUST be
   treated as expired.  "Used" means passing validation at the start of a
   request; a successful validation refreshes the activity timestamp.
2. **Absolute timeout.** A session MUST expire 12 hours after it was created,
   regardless of activity.  Continuous use MUST NOT extend the absolute
   deadline.
3. The two rules are independent: whichever deadline is reached first ends the
   session.
4. An expired session MUST be treated as invalid on the next validation and
   MUST NOT be resurrected by later activity.
5. A request that arrives exactly on a deadline boundary is governed by the
   comparison documented with the configuration constant; deployments SHOULD
   use a strict "greater than the limit" comparison so that the boundary value
   itself is still accepted.

## §5 Concurrent sessions

1. A user MUST have at most 5 concurrent authenticated sessions.
2. When a login would create the sixth session, the service MUST evict the
   existing session that has been idle longest (oldest `last_seen`; ties are
   broken deterministically) and MUST admit the new login.
3. A login that triggers eviction MUST succeed.  Refusing the new login
   because the limit was reached violates this policy.
4. The newly created session MUST NOT itself be evicted by the eviction step
   that its own login triggered.
5. Evicted sessions MUST become invalid immediately.

## §6 Epochs and privilege changes

1. Each user has a monotonically increasing integer called the epoch.
2. Every authenticated session records the epoch that was current when it was
   created.
3. Validation MUST compare the session's recorded epoch with the user's
   current epoch.  A mismatch makes the session invalid.
4. Changing a password, resetting a password, changing roles or permissions,
   disabling an account, or any other event that should force re-login MUST
   advance the user's epoch.
5. After an advance, every session carrying an older epoch MUST be rejected on
   its next validation.  There is no grace period.
6. A session created after the advance MUST be accepted normally.
7. Advancing the epoch SHOULD also revoke the user's remember-me tokens, since
   those tokens can mint a new session.

## §7 Remember-me tokens

1. A remember-me token is a bearer credential.  Only a digest of the token may
   be stored; the plaintext value exists only in the browser.
2. Tokens expire 30 days after they are issued.
3. Every successful redemption MUST rotate the token: the presented token is
   consumed and a new token is returned.
4. Tokens belong to a *family*.  A login that issues a remember-me token starts
   a family; each rotation adds a member to the same family.
5. Presenting a token that has already been redeemed (a *replay*) is evidence
   that the credential leaked.  On detecting a replay the service MUST
   invalidate the entire family — every token descended from the same original
   issuance — and MUST reject the request.
6. The replay signal MUST be observable to the caller (a distinct error), so
   that an application can force re-authentication and alert on it.
7. A token that is unknown, expired, or whose family has been invalidated MUST
   be rejected.  Unknown and revoked tokens MUST be indistinguishable to the
   client.
8. Concurrent redemption of the same token MUST NOT result in one success and
   one benign "not found".  Exactly one redemption may win; every other
   attempt is treated as a replay under rule 5 and invalidates the family.
9. Redemption MUST be safe against concurrent attempts (for example by an
   atomic compare-and-delete in the backing store).

## §8 Cookie attributes

1. The session cookie name is `sessforge_session` and its path is `/`.
2. The cookie MUST be `HttpOnly`.
3. The cookie MUST be `Secure`.
4. The cookie MUST set `SameSite=Lax`.
5. The session cookie SHOULD carry a `Max-Age` aligned with the absolute
   lifetime, so browsers discard it no later than the server does.
6. Logout MUST send an expiring cookie (`Max-Age=0` with an empty value) on
   the same name and path.

## §9 Logout and revocation

1. Logout MUST delete the server-side record; clearing the browser cookie
   alone is insufficient.
2. A logged-out `sid` MUST be rejected by the next validation, even if the
   client kept the cookie.
3. An application MAY revoke a single session, all sessions of one user, or
   all sessions of a user except the current one.  Revocation MUST take effect
   on the next validation.

## §10 Storage contract

1. Storage is pluggable.  The built-in implementation is in-memory and
   thread-safe.
2. A backing store MUST provide atomic session creation (reject duplicate
   identifiers), session read/write/delete, lookup of a user's sessions,
   activity updates, epoch read/write, and remember-token read/write/delete.
3. Operations on unknown identifiers MUST be idempotent where the interface
   says so (deletion in particular).
4. Implementations used in production MUST survive process restarts and shared
   across application instances; the in-memory implementation is for
   development and tests.
