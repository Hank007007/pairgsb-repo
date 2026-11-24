"""OIDC callback handling."""

from __future__ import annotations

import html
from dataclasses import dataclass, field

from ..oidc.jwks import JwksFetcher
from ..oidc.token import TokenExchangeError, TokenTransport, exchange_code
from ..oidc.verify import InvalidTokenError, verify_id_token
from ..providers import ProviderConfig
from ..session.store import SESSION_COOKIE_NAME, Session, SessionError, SessionStore
from .transaction import LoginTransaction, TransactionError, TransactionStore


@dataclass
class CallbackRequest:
    """The parts of an OIDC callback request the handler needs."""

    state: str | None = None
    code: str | None = None
    redirect_uri: str | None = None
    session_id: str | None = None


@dataclass
class CallbackResponse:
    """A framework-neutral HTTP response."""

    status: int
    headers: dict[str, str] = field(default_factory=dict)
    body: str = ""
    session_id: str | None = None

    @property
    def cookies(self) -> dict[str, str]:
        """Return the cookies set by this response."""

        cookies: dict[str, str] = {}
        for header in self.headers.get("set-cookie", "").split("\n"):
            if not header:
                continue
            pair = header.split(";", 1)[0]
            if "=" in pair:
                name, value = pair.split("=", 1)
                cookies[name.strip()] = value.strip()
        return cookies


def _error(status: int, message: str) -> CallbackResponse:
    return CallbackResponse(
        status=status,
        headers={"content-type": "text/html; charset=utf-8"},
        body=f"<!doctype html><html><body><p>{html.escape(message)}</p></body></html>",
    )


def _session_cookie(session_id: str) -> str:
    return f"{SESSION_COOKIE_NAME}={session_id}; Path=/; HttpOnly; SameSite=Lax"


def handle_callback(
    request: CallbackRequest,
    *,
    provider: ProviderConfig,
    transactions: TransactionStore,
    sessions: SessionStore,
    transport: TokenTransport | None = None,
    fetcher: JwksFetcher | None = None,
    now: float | None = None,
) -> CallbackResponse:
    """Process the IdP redirect that completes a login.

    Args:
        request: The callback parameters plus the browser's current session id.
        provider: Configuration of the provider that started the login.
        transactions: Store holding pending login transactions.
        sessions: Store holding anonymous and authenticated sessions.
        transport: Optional token endpoint transport override.
        fetcher: Optional JWKS transport override.
        now: Current UNIX time; defaults to the system clock.

    Returns:
        A :class:`CallbackResponse`.  Successful logins redirect (302) and set
        the session cookie; failed logins return an error response.
    """

    if not request.state:
        return _error(400, "Missing state parameter.")
    if not request.code:
        return _error(400, "Missing authorization code.")

    try:
        transaction: LoginTransaction = transactions.consume(request.state, now=now)
    except TransactionError:
        return _error(400, "Login transaction is not valid.")

    try:
        tokens = exchange_code(
            provider,
            request.code,
            redirect_uri=transaction.redirect_uri,
            transport=transport,
        )
    except TokenExchangeError:
        return CallbackResponse(status=200, body="")

    id_token = tokens["id_token"]
    access_token = tokens.get("access_token")

    try:
        claims = verify_id_token(
            id_token,
            provider,
            access_token=access_token,
            fetcher=fetcher,
            now=now,
        )
    except InvalidTokenError:
        return _error(401, "ID token could not be validated.")

    session: Session
    try:
        session = sessions.promote(request.session_id, claims, now=now)
    except SessionError:
        return _error(400, "The browser session is not available.")

    return CallbackResponse(
        status=302,
        headers={
            "location": "/",
            "set-cookie": _session_cookie(session.id),
        },
        session_id=session.id,
    )
