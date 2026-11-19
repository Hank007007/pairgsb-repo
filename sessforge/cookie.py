"""Session cookie construction and logout cleanup.

The cookie is the only thing the client sees; every cookie issued here is
paired with a server-side record created by :mod:`sessforge.lifecycle`.
"""

from __future__ import annotations

from dataclasses import dataclass

from .store import SessionStore

SESSION_COOKIE_NAME = "sessforge_session"
SESSION_COOKIE_PATH = "/"


@dataclass(frozen=True)
class Cookie:
    """A browser cookie, rendered by :meth:`to_header`."""

    name: str
    value: str
    path: str = SESSION_COOKIE_PATH
    max_age: int | None = None
    http_only: bool = True
    secure: bool = True
    same_site: str = "Lax"

    def to_header(self) -> str:
        parts = [f"{self.name}={self.value}", f"Path={self.path}"]
        if self.max_age is not None:
            parts.append(f"Max-Age={self.max_age}")
        if self.http_only:
            parts.append("HttpOnly")
        if self.secure:
            parts.append("Secure")
        if self.same_site:
            parts.append(f"SameSite={self.same_site}")
        return "; ".join(parts)


def session_cookie(sid: str, *, max_age: int | None = None) -> Cookie:
    """Build the cookie that carries session identifier ``sid``."""
    return Cookie(name=SESSION_COOKIE_NAME, value=sid, max_age=max_age)


def clearing_cookie() -> Cookie:
    """Build the cookie that tells the browser to discard the session cookie."""
    return Cookie(name=SESSION_COOKIE_NAME, value="", max_age=0)


def logout(store: SessionStore, sid: str) -> Cookie:
    """End the session identified by ``sid``.

    The server-side record is removed and the returned cookie instructs the
    browser to drop its copy.
    """
    store.delete_session(sid)
    return clearing_cookie()
