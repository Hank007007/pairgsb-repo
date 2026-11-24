from exchangefence import policy
from exchangefence.http.handler import handle_exchange


def test_handle_exchange_returns_token_response(make_token, exchange_request):
    subject = make_token(scope="docs:read docs:write")
    status, body = handle_exchange(exchange_request(subject, requested_scope="docs:read"))

    assert status == 200
    assert set(body) == {
        "access_token",
        "issued_token_type",
        "token_type",
        "expires_in",
        "scope",
        "refresh_token",
    }
    assert body["issued_token_type"] == policy.ACCESS_TOKEN_TYPE
    assert body["token_type"] == "Bearer"
    assert body["scope"] == "docs:read"
