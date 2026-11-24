from exchangefence import policy
from exchangefence.exchange.service import exchange, refresh
from exchangefence.tokens import codec


def test_exchange_returns_a_token_for_the_requested_scope(make_token, exchange_request):
    subject = make_token(scope="docs:read docs:write")
    result = exchange(exchange_request(subject, requested_scope="docs:read"))

    assert result["scope"] == "docs:read"
    assert result["token_type"] == "Bearer"
    assert result["issued_token_type"] == policy.ACCESS_TOKEN_TYPE
    assert result["expires_in"] > 0
    assert result["refresh_token"]

    claims = codec.decode(result["access_token"])
    assert claims["iss"] == policy.ISSUER
    assert claims["sub"] == "user-1"
    assert claims["aud"] == "docs-api"
    assert claims["scope"] == "docs:read"


def test_exchange_then_refresh_returns_new_tokens(make_token, exchange_request):
    subject = make_token(scope="docs:read docs:write")
    result = exchange(exchange_request(subject, requested_scope="docs:read"))

    refreshed = refresh(
        {
            "grant_type": "refresh_token",
            "client_id": "svc-a",
            "refresh_token": result["refresh_token"],
        }
    )

    assert refreshed["access_token"]
    assert refreshed["refresh_token"] != result["refresh_token"]
    claims = codec.decode(refreshed["access_token"])
    assert claims["sub"] == "user-1"
    assert claims["aud"] == "docs-api"
