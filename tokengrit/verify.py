"""Token verification.

``verify_token`` is the only public way to turn a presented token into a
decision.  The reason codes it returns are part of the API contract described
in ``docs/token-format.md``.
"""

from __future__ import annotations

import time
from dataclasses import dataclass, field

from . import audit, hashing, token
from .store import Store, TokenRecord

OK = "ok"
INVALID = "invalid_token"
REVOKED = "revoked"
EXPIRED = "expired"
INSUFFICIENT_SCOPE = "insufficient_scope"


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
    reason: str,
    detail: str,
    *,
    raw: str,
    token_id: str | None = None,
    family_id: str | None = None,
    now: float,
) -> VerifyResult:
    audit.record(
        store.connection,
        audit.TOKEN_VERIFY_FAILED,
        token_id=token_id,
        family_id=family_id,
        token=raw,
        now=now,
        reason=reason,
        detail=detail,
    )
    return VerifyResult(False, reason, token_id, family_id, ())


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
    return derived.hex() == params.digest.hex()


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
        return _fail(store, INVALID, "malformed", raw=str(raw), now=ts)

    record = store.lookup_by_prefix(parsed.token_id)
    if record is None:
        return _fail(
            store,
            "unknown_prefix",
            "unknown_prefix",
            raw=raw,
            token_id=parsed.token_id,
            now=ts,
        )

    if not _digest_matches(parsed.secret, record):
        return _fail(
            store,
            "bad_secret",
            "bad_secret",
            raw=raw,
            token_id=parsed.token_id,
            family_id=record.family_id,
            now=ts,
        )

    if record.revoked_at is not None:
        return _fail(
            store,
            REVOKED,
            "revoked",
            raw=raw,
            token_id=record.token_id,
            family_id=record.family_id,
            now=ts,
        )

    if record.expires_at is not None and ts >= record.expires_at:
        return _fail(
            store,
            EXPIRED,
            "expired",
            raw=raw,
            token_id=record.token_id,
            family_id=record.family_id,
            now=ts,
        )

    if record.grace_expires_at is not None and ts >= record.grace_expires_at:
        return _fail(
            store,
            EXPIRED,
            "grace_expired",
            raw=raw,
            token_id=record.token_id,
            family_id=record.family_id,
            now=ts,
        )

    if required_scope is not None and required_scope not in record.scopes:
        return _fail(
            store,
            INSUFFICIENT_SCOPE,
            "insufficient_scope",
            raw=raw,
            token_id=record.token_id,
            family_id=record.family_id,
            now=ts,
        )

    audit.record(
        store.connection,
        audit.TOKEN_VERIFIED,
        token_id=record.token_id,
        family_id=record.family_id,
        token=raw,
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
