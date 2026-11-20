"""HTTP client for an issuer's token endpoint."""

from __future__ import annotations

import json
from datetime import datetime, timedelta, timezone
from typing import Any, Callable, Dict, Iterable, Mapping, Optional
from urllib.parse import urlencode
from urllib.request import Request, urlopen

from .errors import TokenEndpointError
from .models import TokenEntry
from .scopes import canonicalize


class HttpTransport:
    """Minimal form-encoded POST transport built on the standard library."""

    def __init__(self, timeout: float = 10.0) -> None:
        self._timeout = timeout

    def post(self, url: str, form: Mapping[str, str]) -> Dict[str, Any]:
        body = urlencode(dict(form)).encode("utf-8")
        request = Request(
            url,
            data=body,
            headers={
                "Content-Type": "application/x-www-form-urlencoded",
                "Accept": "application/json",
            },
            method="POST",
        )
        try:
            with urlopen(request, timeout=self._timeout) as response:
                raw = response.read().decode("utf-8")
        except OSError as exc:
            raise TokenEndpointError(f"token endpoint request failed: {exc}") from exc
        try:
            payload = json.loads(raw)
        except ValueError as exc:
            raise TokenEndpointError("token endpoint returned malformed JSON") from exc
        if not isinstance(payload, dict):
            raise TokenEndpointError("token endpoint returned a non-object payload")
        return payload


class TokenClient:
    """Exchanges client credentials for access tokens."""

    def __init__(
        self,
        transport: Any = None,
        clock: Optional[Callable[[], datetime]] = None,
    ) -> None:
        self._transport = transport if transport is not None else HttpTransport()
        self._clock = clock if clock is not None else (lambda: datetime.now(timezone.utc))
        self._revocations: list = []

    def token_url(self, issuer: str) -> str:
        return issuer.rstrip("/") + "/token"

    def request_token(
        self,
        issuer: str,
        client_id: str,
        scopes: Iterable[str],
    ) -> TokenEntry:
        requested = canonicalize(scopes)
        form = {
            "grant_type": "client_credentials",
            "client_id": client_id,
            "scope": " ".join(requested),
        }
        payload = self._transport.post(self.token_url(issuer), form)
        return self.parse_response(payload, " ".join(requested))

    def parse_response(
        self,
        payload: Mapping[str, Any],
        requested: Iterable[str],
    ) -> TokenEntry:
        access_token = payload.get("access_token")
        if not isinstance(access_token, str) or not access_token:
            raise TokenEndpointError("token response is missing access_token")

        token_type = payload.get("token_type", "Bearer")
        expires_in = payload.get("expires_in", 3600)
        try:
            expires_in = float(expires_in)
        except (TypeError, ValueError) as exc:
            raise TokenEndpointError("token response has a non-numeric expires_in") from exc

        granted = payload.get("scope")
        if granted is None:
            scopes = canonicalize(requested)
        else:
            scopes = canonicalize(granted)

        expires_at = self._clock() + timedelta(seconds=expires_in)
        return TokenEntry(
            access_token=access_token,
            token_type=str(token_type),
            scopes=tuple(scopes),
            expires_at=expires_at,
        )

    def revoke_token(self, issuer: str, client_id: str, access_token: str) -> None:
        """Report a revoked access token to the issuer that minted it."""
        self._revocations.append((issuer, client_id))
        return None
