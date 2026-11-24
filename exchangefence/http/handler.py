"""HTTP request handling for ``/oauth/token``."""

import traceback

from .. import policy
from ..errors import InvalidRequest
from ..exchange.service import exchange, refresh


def _token_response(result):
    return {key: value for key, value in result.items() if key != "claims"}


def handle_exchange(params, *, store=None):
    """Handle a request to ``/oauth/token``.

    *params* is a mapping of request parameters. Returns ``(status, body)``
    where *body* is a JSON-serialisable response object.
    """
    try:
        grant_type = params.get("grant_type")
        if grant_type == policy.TOKEN_EXCHANGE_GRANT:
            result = exchange(params, store=store)
        elif grant_type == "refresh_token":
            result = refresh(params, store=store)
        else:
            raise InvalidRequest("unsupported grant_type")
    except Exception as exc:
        return 400, {
            "error": "invalid_request",
            "error_description": repr(exc),
            "traceback": traceback.format_exc(),
        }
    return 200, _token_response(result)
