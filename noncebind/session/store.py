"""Session storage for anonymous and authenticated sessions."""

from __future__ import annotations

import secrets
import time
from dataclasses import dataclass, field
from typing import Any, Callable

SESSION_COOKIE_NAME = "nb_session"


class SessionError(ValueError):
    """Base class for session errors."""


class UnknownSessionError(SessionError):
    """Raised when a session id is not present in the store."""


@dataclass
class Session:
    """One server-side session record."""

    id: str
    authenticated: bool
    created_at: float
    subject: str | None = None
    claims: dict[str, Any] = field(default_factory=dict)


@dataclass
class SessionStore:
    """In-memory session store."""

    clock: Callable[[], float] = time.time
    _sessions: dict[str, Session] = field(default_factory=dict)

    def create_anonymous(self, *, now: float | None = None) -> Session:
        """Create a session for a visitor who has not logged in yet."""

        created_at = self.clock() if now is None else float(now)
        session = Session(
            id="anon_" + secrets.token_urlsafe(24),
            authenticated=False,
            created_at=created_at,
        )
        self._sessions[session.id] = session
        return session

    def promote(
        self,
        session_id: str,
        claims: dict[str, Any],
        *,
        now: float | None = None,
    ) -> Session:
        """Attach an authenticated identity to ``session_id``."""

        session = self._sessions.get(session_id)
        if session is None:
            raise UnknownSessionError(session_id)
        promoted_at = self.clock() if now is None else float(now)
        session.authenticated = True
        session.subject = claims.get("sub")
        session.claims = dict(claims)
        session.created_at = promoted_at
        return session

    def get(self, session_id: str | None) -> Session | None:
        """Return the live session for ``session_id``, if any."""

        if not session_id:
            return None
        return self._sessions.get(session_id)

    def revoke(self, session_id: str) -> None:
        """Remove a session from the store."""

        self._sessions.pop(session_id, None)

    def __len__(self) -> int:
        return len(self._sessions)
