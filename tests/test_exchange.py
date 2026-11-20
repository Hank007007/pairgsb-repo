"""Tests for the token exchange flow.

These exercise the ordinary single-hop path: an access token subject, a
requested scope equal to the scope the client is registered for, and a
single string audience.
"""

from __future__ import annotations

import pytest

from scopebraid import tokens
from scopebraid.errors import ExchangeError
from scopebraid.exchange import ACCESS_TOKEN_TYPE, ExchangeRequest, TokenExchangeService


def _subject_token(
    signing_key: bytes,
    clock: float,
    *,
    sub: str = "user-1",
    aud: object = "api://warehouse",
    scope: str = "datasets:read files:read",
    jti: str = "subject-jti",
    exp_offset: float = 900.0,
) -> str:
    return tokens.encode(
        {
            "iss": "scopebraid",
            "sub": sub,
            "aud": aud,
            "scope": scope,
            "token_use": "access",
            "iat": clock,
            "nbf": clock,
            "exp": clock + exp_offset,
            "jti": jti,
        },
        signing_key,
    )


def test_single_hop_access_token(
    service: TokenExchangeService, signing_key: bytes, clock: float
) -> None:
    subject = _subject_token(signing_key, clock)
    result = service.exchange(
        ExchangeRequest(
            subject_token=subject,
            audience="api://warehouse",
            scope="datasets:read files:read",
            client_id="svc-analytics",
        )
    )

    assert result.token_type == "Bearer"
    assert result.issued_token_type == ACCESS_TOKEN_TYPE
    assert result.scope == "datasets:read files:read"
    assert result.expires_in == 300

    issued = tokens.verify(result.access_token, signing_key, now=clock)
    assert issued["sub"] == "user-1"
    assert issued["aud"] == "api://warehouse"
    assert issued["scope"] == "datasets:read files:read"
    assert issued["client_id"] == "svc-analytics"
    assert issued["token_use"] == "access"
    assert issued["iss"] == "scopebraid"
    assert issued["act"]["sub"] == "svc-analytics"
    assert issued["act"]["jti"] == "subject-jti"


def test_registered_audience_and_string_aud(
    service: TokenExchangeService, signing_key: bytes, clock: float
) -> None:
    subject = _subject_token(signing_key, clock)
    result = service.exchange(
        ExchangeRequest(
            subject_token=subject,
            audience="api://warehouse",
            scope="datasets:read",
            client_id="svc-analytics",
        )
    )
    issued = tokens.verify(result.access_token, signing_key, now=clock)
    assert issued["aud"] == "api://warehouse"


def test_lifetime_is_capped_by_subject_expiry(
    service: TokenExchangeService, signing_key: bytes, clock: float
) -> None:
    subject = _subject_token(signing_key, clock, exp_offset=120.0)
    result = service.exchange(
        ExchangeRequest(
            subject_token=subject,
            audience="api://warehouse",
            scope="datasets:read",
            client_id="svc-analytics",
        )
    )
    assert result.expires_in == 120


def test_unknown_client_is_rejected(
    service: TokenExchangeService, signing_key: bytes, clock: float
) -> None:
    subject = _subject_token(signing_key, clock)
    with pytest.raises(ExchangeError) as excinfo:
        service.exchange(
            ExchangeRequest(
                subject_token=subject,
                audience="api://warehouse",
                scope="datasets:read",
                client_id="svc-unknown",
            )
        )
    assert excinfo.value.error == "invalid_client"


def test_revoked_subject_token_is_rejected(
    service: TokenExchangeService, signing_key: bytes, clock: float
) -> None:
    subject = _subject_token(signing_key, clock)
    service.revoked.add("subject-jti")
    with pytest.raises(ExchangeError) as excinfo:
        service.exchange(
            ExchangeRequest(
                subject_token=subject,
                audience="api://warehouse",
                scope="datasets:read",
                client_id="svc-analytics",
            )
        )
    assert excinfo.value.error == "invalid_request"


def test_expired_subject_token_is_rejected(
    service: TokenExchangeService, signing_key: bytes, clock: float
) -> None:
    subject = _subject_token(signing_key, clock, exp_offset=-10.0)
    with pytest.raises(ExchangeError) as excinfo:
        service.exchange(
            ExchangeRequest(
                subject_token=subject,
                audience="api://warehouse",
                scope="datasets:read",
                client_id="svc-analytics",
            )
        )
    assert excinfo.value.error == "invalid_request"
