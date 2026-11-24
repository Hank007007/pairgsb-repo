"""Authorization request construction."""

from __future__ import annotations

import urllib.parse

from ..providers import ProviderConfig
from .transaction import LoginTransaction


def build_authorization_url(
    provider: ProviderConfig,
    transaction: LoginTransaction,
    *,
    prompt: str | None = None,
) -> str:
    """Build the URL the browser is sent to when a login starts.

    The returned URL carries the transaction's ``state`` and ``nonce`` and is
    bound to the redirect URI recorded on the transaction.
    """

    params = {
        "response_type": "code",
        "client_id": provider.client_id,
        "redirect_uri": transaction.redirect_uri,
        "scope": provider.scope,
        "state": transaction.state,
        "nonce": transaction.nonce,
    }
    if transaction.max_age is not None:
        params["max_age"] = str(transaction.max_age)
    if prompt is not None:
        params["prompt"] = prompt
    query = urllib.parse.urlencode(params, quote_via=urllib.parse.quote)
    separator = "&" if urllib.parse.urlparse(provider.authorization_endpoint).query else "?"
    return provider.authorization_endpoint + separator + query
