"""Tests covering the happy path of a single login."""

from __future__ import annotations

import json
import urllib.parse

import pytest

from noncebind.login.authorize import build_authorization_url
from noncebind.login.callback import CallbackRequest, handle_callback
from noncebind.login.transaction import TransactionStore
from noncebind.oidc.jwks import reset_cache
from noncebind.oidc.verify import verify_id_token
from noncebind.providers import get_provider, provider_names
from noncebind.session.store import SESSION_COOKIE_NAME, SessionStore

from .helpers import load_fixture

PROVIDER = get_provider("okta-dev")


@pytest.fixture(autouse=True)
def clear_jwks_cache():
    reset_cache()
    yield
    reset_cache()


@pytest.fixture()
def fixture() -> dict:
    return load_fixture("okta_idp.json")


def test_registry_exposes_all_providers():
    assert set(provider_names()) == {
        "okta-dev",
        "azure-ad",
        "keycloak-eu",
        "auth0-partner",
    }


def test_id_token_is_accepted_for_valid_input(fixture):
    def fetcher(_uri):
        return fixture["jwks"]

    claims = verify_id_token(
        fixture["id_token"],
        PROVIDER,
        access_token=fixture["access_token"],
        nonce=fixture["nonce"],
        max_age=fixture["max_age"],
        now=fixture["now"],
        fetcher=fetcher,
    )

    assert claims["sub"] == "user-1234"
    assert claims["iss"] == fixture["issuer"]
    assert claims["nonce"] == fixture["nonce"]


def test_authorization_url_carries_state_and_nonce(fixture):
    transactions = TransactionStore()
    transaction = transactions.issue(
        PROVIDER,
        PROVIDER.redirect_uri,
        max_age=fixture["max_age"],
        now=fixture["now"],
    )

    url = build_authorization_url(PROVIDER, transaction)
    query = urllib.parse.parse_qs(urllib.parse.urlparse(url).query)

    assert query["state"] == [transaction.state]
    assert query["nonce"] == [transaction.nonce]
    assert query["redirect_uri"] == [PROVIDER.redirect_uri]
    assert query["client_id"] == [PROVIDER.client_id]
    assert query["max_age"] == [str(fixture["max_age"])]


def test_callback_completes_login_and_sets_cookie(fixture):
    transactions = TransactionStore()
    sessions = SessionStore()
    anonymous = sessions.create_anonymous(now=fixture["now"])
    transaction = transactions.issue(
        PROVIDER,
        PROVIDER.redirect_uri,
        max_age=fixture["max_age"],
        nonce=fixture["nonce"],
        now=fixture["now"],
    )

    def fetch_jwks(_uri):
        return fixture["jwks"]

    def transport(_endpoint, _form):
        return {
            "access_token": fixture["access_token"],
            "id_token": fixture["id_token"],
            "token_type": "Bearer",
        }

    response = handle_callback(
        CallbackRequest(
            state=transaction.state,
            code="authorization-code",
            redirect_uri=PROVIDER.redirect_uri,
            session_id=anonymous.id,
        ),
        provider=PROVIDER,
        transactions=transactions,
        sessions=sessions,
        transport=transport,
        fetcher=fetch_jwks,
        now=fixture["now"],
    )

    assert response.status == 302
    assert response.headers["location"] == "/"
    cookie = response.cookies[SESSION_COOKIE_NAME]
    session = sessions.get(cookie)
    assert session is not None
    assert session.authenticated is True
    assert session.subject == "user-1234"


def test_anonymous_session_is_not_authenticated(fixture):
    sessions = SessionStore()
    anonymous = sessions.create_anonymous(now=fixture["now"])

    assert anonymous.authenticated is False
    assert sessions.get(anonymous.id) is anonymous


def test_fixture_token_has_expected_claims(fixture):
    import base64

    header_segment, claims_segment, _ = fixture["id_token"].split(".")
    header = json.loads(base64.urlsafe_b64decode(header_segment + "=="))
    claims = json.loads(base64.urlsafe_b64decode(claims_segment + "=="))

    assert header["alg"] == "RS256"
    assert header["kid"] == "okta-2026-01"
    assert claims["iss"] == fixture["issuer"]
    assert claims["aud"] == fixture["client_id"]
