"""OIDC token endpoint client."""

from __future__ import annotations

import json
import urllib.error
import urllib.parse
import urllib.request
from collections.abc import Mapping
from typing import Any, Callable

from ..providers import ProviderConfig

TokenTransport = Callable[[str, Mapping[str, str]], Mapping[str, Any]]


class TokenExchangeError(RuntimeError):
    """Raised when an authorization code cannot be exchanged for tokens."""


def _post_form(url: str, form: Mapping[str, str], *, timeout: float = 10.0) -> Mapping[str, Any]:
    body = urllib.parse.urlencode(dict(form)).encode("ascii")
    request = urllib.request.Request(
        url,
        data=body,
        headers={
            "Accept": "application/json",
            "Content-Type": "application/x-www-form-urlencoded",
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            payload = response.read()
    except (urllib.error.URLError, OSError) as exc:
        raise TokenExchangeError("token endpoint request failed") from exc
    try:
        document = json.loads(payload.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise TokenExchangeError("token endpoint returned invalid JSON") from exc
    if not isinstance(document, dict):
        raise TokenExchangeError("token endpoint returned an unexpected payload")
    return document


def exchange_code(
    provider: ProviderConfig,
    code: str,
    *,
    redirect_uri: str,
    transport: TokenTransport | None = None,
) -> dict[str, Any]:
    """Exchange an authorization code for a token set.

    Args:
        provider: The provider that issued the code.
        code: The authorization code from the callback.
        redirect_uri: The redirect URI sent with the authorization request.
        transport: Optional replacement for the HTTP POST used in production.

    Returns:
        The decoded token endpoint response.  It contains at least
        ``access_token`` and ``id_token`` on success.

    Raises:
        TokenExchangeError: if the endpoint rejects the request or returns an
            error payload.
    """

    if not isinstance(code, str) or not code:
        raise TokenExchangeError("authorization code is missing")

    form = {
        "grant_type": "authorization_code",
        "code": code,
        "redirect_uri": redirect_uri,
        "client_id": provider.client_id,
        "client_secret": provider.client_secret,
    }
    send = transport if transport is not None else _post_form
    try:
        payload = send(provider.token_endpoint, form)
    except TokenExchangeError:
        raise
    except Exception as exc:  # noqa: BLE001 - transport implementations vary
        raise TokenExchangeError("token endpoint request failed") from exc

    if not isinstance(payload, Mapping):
        raise TokenExchangeError("token endpoint returned an unexpected payload")
    if payload.get("error"):
        description = payload.get("error_description") or payload["error"]
        raise TokenExchangeError(str(description))
    if not payload.get("id_token"):
        raise TokenExchangeError("token response does not contain an id_token")
    return dict(payload)
