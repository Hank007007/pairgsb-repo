"""HTTP request handling for ``/oauth/token``."""

from .. import policy
from ..errors import ExchangeError, InvalidRequest
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
    except ExchangeError as exc:
        return exc.status, {
            "error": exc.error,
            "error_description": exc.description or exc.error,
        }
    except Exception:
        return 500, {
            "error": "server_error",
            "error_description": "an internal error occurred",
        }
    return 200, _token_response(result)
