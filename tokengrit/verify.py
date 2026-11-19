"""Token verification.

``verify_token`` is the only public way to turn a presented token into a
decision.  The reason codes it returns are part of the API contract described
in ``docs/token-format.md``.
"""

from __future__ import annotations

import hmac
import time
from dataclasses import dataclass, field

from . import audit, hashing, token
from .store import Store, TokenRecord

OK = "ok"
INVALID = "invalid_token"
REVOKED = "revoked"
EXPIRED = "expired"
INSUFFICIENT_SCOPE = "insufficient_scope"

CATEGORY_MALFORMED = "malformed"
CATEGORY_UNKNOWN_PREFIX = "unknown_prefix"
CATEGORY_BAD_SECRET = "bad_secret"


@dataclass(frozen=True)
class VerifyResult:
    """The outcome of a verification attempt."""

    ok: bool
    reason: str
    token_id: str | None = None
    family_id: str | None = None
    scopes: tuple[str, ...] = field(default_factory=tuple)


def _fail(
    store: Store,
    *,
    token_id: str | None = None,
    family_id: str | None = None,
    detail: str,
    now: float,
) -> VerifyResult:
    audit.record(
        store.connection,
        audit.TOKEN_VERIFY_FAILED,
        token_id=token_id,
        family_id=family_id,
        now=now,
        reason=INVALID,
        detail=detail,
    )
    return VerifyResult(False, INVALID, None, None, ())


def _digest_matches(secret: str, record: TokenRecord) -> bool:
    params = hashing.parse_encoded(record.secret_hash)
    derived = hashing.derive(
        secret,
        params.salt,
        n=params.n,
        r=params.r,
        p=params.p,
        dklen=len(params.digest),
    )
    return hmac.compare_digest(derived, params.digest)


def match_record(
    candidates: list[TokenRecord], secret: str
) -> TokenRecord | None:
    """Return the candidate whose stored hash matches ``secret``.

    Every candidate is compared independently; a single matching record is
    sufficient to authenticate the token.  A record whose stored hash is not
    a well-formed scrypt representation is a storage integrity error for
    that record and can never authenticate.
    """
    for record in candidates:
        try:
            if _digest_matches(secret, record):
                return record
        except ValueError:
            continue
    return None


def verify_token(
    store: Store,
    raw: object,
    *,
    required_scope: str | None = None,
    now: float | None = None,
) -> VerifyResult:
    """Verify ``raw`` and report whether it may be used."""
    ts = time.time() if now is None else now
    parsed = token.parse_token(raw)
    if parsed is None:
        return _fail(
            store,
            token_id=None,
            family_id=None,
            detail=CATEGORY_MALFORMED,
            now=ts,
        )

    candidates = store.lookup_by_prefix(parsed.token_id)
    if not candidates:
        return _fail(
            store,
            token_id=parsed.token_id,
            family_id=None,
            detail=CATEGORY_UNKNOWN_PREFIX,
            now=ts,
        )

    record = match_record(candidates, parsed.secret)
    if record is None:
        return _fail(
            store,
            token_id=parsed.token_id,
            family_id=candidates[0].family_id,
            detail=CATEGORY_BAD_SECRET,
            now=ts,
        )

    if record.revoked_at is not None:
        audit.record(
            store.connection,
            audit.TOKEN_VERIFY_FAILED,
            token_id=record.token_id,
            family_id=record.family_id,
            now=ts,
            reason=REVOKED,
            detail="revoked",
        )
        return VerifyResult(False, REVOKED, record.token_id, record.family_id, ())

    if record.expires_at is not None and ts >= record.expires_at:
        audit.record(
            store.connection,
            audit.TOKEN_VERIFY_FAILED,
            token_id=record.token_id,
            family_id=record.family_id,
            now=ts,
            reason=EXPIRED,
            detail="expired",
        )
        return VerifyResult(False, EXPIRED, record.token_id, record.family_id, ())

    if record.grace_expires_at is not None and ts >= record.grace_expires_at:
        audit.record(
            store.connection,
            audit.TOKEN_VERIFY_FAILED,
            token_id=record.token_id,
            family_id=record.family_id,
            now=ts,
            reason=EXPIRED,
            detail="grace_expired",
        )
        return VerifyResult(False, EXPIRED, record.token_id, record.family_id, ())

    if required_scope is not None and required_scope not in record.scopes:
        audit.record(
            store.connection,
            audit.TOKEN_VERIFY_FAILED,
            token_id=record.token_id,
            family_id=record.family_id,
            now=ts,
            reason=INSUFFICIENT_SCOPE,
            detail="insufficient_scope",
        )
        return VerifyResult(
            False, INSUFFICIENT_SCOPE, record.token_id, record.family_id, ()
        )

    audit.record(
        store.connection,
        audit.TOKEN_VERIFIED,
        token_id=record.token_id,
        family_id=record.family_id,
        now=ts,
        scopes=list(record.scopes),
    )
    return VerifyResult(
        ok=True,
        reason=OK,
        token_id=record.token_id,
        family_id=record.family_id,
        scopes=record.scopes,
    )
