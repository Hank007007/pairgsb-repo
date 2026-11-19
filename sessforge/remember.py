"""Long-lived remember-me tokens.

A remember-me token is a bearer credential that survives an expired session.
Only a digest of the token is stored.  Redeeming a token consumes it and hands
out a successor, so the value the browser holds changes on every use.

The caller is responsible for the surrounding session work: this module only
tracks token state and rotation.
"""

from __future__ import annotations

import hashlib
import secrets
from dataclasses import dataclass

from .store import RememberToken, SessionStore

#: Lifetime of one token, in seconds (30 days).
REMEMBER_TOKEN_TTL_SECONDS = 30 * 24 * 60 * 60

#: Length in bytes of the random component of a token.
TOKEN_BYTES = 32


def _hash(token: str) -> str:
    return hashlib.sha256(token.encode("utf-8")).hexdigest()


@dataclass(frozen=True)
class IssuedToken:
    """A freshly minted token handed to the client."""

    token: str
    user_id: str
    expires_at: float


@dataclass(frozen=True)
class RedeemedToken:
    """The result of a successful redemption."""

    token: str
    user_id: str
    expires_at: float


class RememberError(Exception):
    """Base class for remember-me failures."""


class TokenNotFound(RememberError):
    """The presented token is not stored."""


class TokenExpired(RememberError):
    """The presented token exists but is past its expiry."""


class TokenReplay(RememberError):
    """The presented token was already redeemed once.

    A replay means the bearer credential leaked: its whole family has been
    invalidated and the caller must force re-authentication.
    """


def _new_family_id() -> str:
    return secrets.token_urlsafe(TOKEN_BYTES)


def issue(store: SessionStore, user_id: str, now: float) -> IssuedToken:
    """Mint a new remember-me token for ``user_id``."""
    return _issue(store, user_id, now, family_id=_new_family_id())


def _issue(store: SessionStore, user_id: str, now: float, *, family_id: str) -> IssuedToken:
    """Mint a token belonging to ``family_id``."""
    token = secrets.token_urlsafe(TOKEN_BYTES)
    expires_at = now + REMEMBER_TOKEN_TTL_SECONDS
    token_hash = _hash(token)
    store.put_remember_token(
        token_hash,
        RememberToken(
            token_hash=token_hash,
            user_id=user_id,
            issued_at=now,
            expires_at=expires_at,
            family_id=family_id,
        ),
    )
    return IssuedToken(token=token, user_id=user_id, expires_at=expires_at)


def redeem(store: SessionStore, token: str, now: float) -> RedeemedToken:
    """Exchange a valid token for a successor.

    The presented token is consumed atomically: concurrent redemptions of the
    same value have exactly one winner and every loser is treated as a replay.

    Raises :class:`TokenNotFound` when the token is unknown or its family has
    been revoked, :class:`TokenReplay` when the token was already redeemed
    once, and :class:`TokenExpired` when it is past its expiry.
    """
    token_hash = _hash(token)
    record = store.pop_remember_token(token_hash)
    if record is None:
        family_id = store.get_redeemed_remember_family(token_hash)
        if family_id is not None:
            store.invalidate_remember_family(family_id)
            raise TokenReplay(token_hash)
        raise TokenNotFound(token_hash)

    if now > record.expires_at:
        store.mark_remember_token_redeemed(token_hash, record.family_id)
        raise TokenExpired(token_hash)

    store.mark_remember_token_redeemed(token_hash, record.family_id)
    successor = _issue(store, record.user_id, now, family_id=record.family_id)
    return RedeemedToken(
        token=successor.token,
        user_id=successor.user_id,
        expires_at=successor.expires_at,
    )


def revoke_user_tokens(store: SessionStore, user_id: str) -> int:
    """Drop every stored token belonging to ``user_id`` and return the count."""
    tokens = store.list_remember_tokens(user_id)
    for item in tokens:
        store.delete_remember_token(item.token_hash)
    return len(tokens)
