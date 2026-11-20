"""Audit trail for token cache activity.

The audit trail is consumed by the platform security team: it has to explain
who asked for which scopes without ever exposing credential material.
"""

from __future__ import annotations

import logging

logger = logging.getLogger("scopevault.audit")


def log_cache_hit(issuer: str, client_id: str, scopes) -> None:
    logger.info(
        "token cache hit issuer=%s client=%s scopes=%s",
        issuer,
        client_id,
        " ".join(scopes),
    )


def log_fetch(issuer: str, client_id: str, scopes, token: str | None = None) -> None:
    reference = token[:12] if token else "-"
    logger.info(
        "token fetch issuer=%s client=%s scopes=%s outcome=ok ref=%s",
        issuer,
        client_id,
        " ".join(scopes),
        reference,
    )
    if token:
        logger.debug(
            "token material issuer=%s client=%s token=%s",
            issuer,
            client_id,
            token,
        )


def log_revoke(issuer: str, client_id: str, removed: int) -> None:
    logger.info(
        "token revoke issuer=%s client=%s removed=%d",
        issuer,
        client_id,
        removed,
    )
