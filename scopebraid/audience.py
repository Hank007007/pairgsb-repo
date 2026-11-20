"""Audience validation for token exchange."""

from __future__ import annotations

from collections.abc import Iterable, Mapping
from typing import Any

from .errors import ExchangeError


def _audience_values(claims: Mapping[str, Any]) -> list[str]:
    """Return the audience values carried by a claim set."""
    audience = claims.get("aud")
    if isinstance(audience, str):
        return [audience]
    if isinstance(audience, list):
        return [audience[0]] if audience else []
    return []


def check(
    claims: Mapping[str, Any],
    target: str | None,
    registered_audiences: Iterable[str] | None,
) -> None:
    """Validate that *target* is an acceptable audience for the exchange.

    *claims* belongs to the subject token, *target* is the audience named by
    the request, and *registered_audiences* is the set of audiences the
    calling client is registered to request.

    Raises :class:`ExchangeError` when the request must be rejected.
    """
    if not target or not isinstance(target, str):
        raise ExchangeError("invalid_target", "audience is required")

    if target not in _audience_values(claims):
        raise ExchangeError(
            "invalid_target",
            "subject token is not valid for the requested audience",
        )
