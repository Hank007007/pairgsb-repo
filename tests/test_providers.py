"""Vendor adapters, exercised one call at a time against a recorded session."""

from __future__ import annotations

import json
import os

from rotgraph.providers.objstore import ObjStoreProvider
from rotgraph.providers.paygate import PaygateProvider
from rotgraph.providers.signer import SignerProvider
from rotgraph.providers.smsgw import SmsGatewayProvider

from .conftest import FakeResponse, FakeSession

HERE = os.path.dirname(os.path.abspath(__file__))
SAMPLES = os.path.join(os.path.dirname(HERE), "testdata", "providers")


def sample(name: str) -> dict:
    with open(os.path.join(SAMPLES, name), "r", encoding="utf-8") as handle:
        return json.load(handle)


def test_signer_create_returns_the_minted_key():
    session = FakeSession({"/v1/keys": [FakeResponse(200, sample("signer-create.json"))]})
    provider = SignerProvider(
        "signer", {"endpoint": "https://signer.example.test", "secret": "svc"}, session=session
    )

    material = provider.create({"name": "signer.prod", "service": "signer", "slot": "prod"})

    assert material["id"] == "sig_new_9f2"
    assert material["secret"] == "sk-live-sig-9f2c41"
    call = session.calls[-1]
    assert call.method == "POST"
    assert call.url == "https://signer.example.test/v1/keys"
    assert call.kwargs["json"] == {"slot": "prod"}


def test_signer_revoke_reports_success():
    session = FakeSession({"/v1/keys/sig_old_1/revoke": [FakeResponse(200, sample("signer-revoke.json"))]})
    provider = SignerProvider(
        "signer", {"endpoint": "https://signer.example.test", "secret": "svc"}, session=session
    )

    answer = provider.revoke(
        {"name": "signer.prod", "slot": "prod"}, "sig_old_1", keep_until="2026-11-25T10:00:12Z"
    )

    assert answer["ok"] is True
    call = session.calls[-1]
    assert call.method == "POST"
    assert call.url == "https://signer.example.test/v1/keys/sig_old_1/revoke"


def test_objstore_create_speaks_the_credentials_api():
    session = FakeSession({"/api/credentials": [FakeResponse(200, sample("objstore-create.json"))]})
    provider = ObjStoreProvider(
        "objstore", {"endpoint": "https://objstore.example.test", "secret": "svc"}, session=session
    )

    material = provider.create({"name": "objstore.prod", "service": "objstore", "slot": "prod"})

    known = {c["id"]: c["secret"] for c in sample("objstore-create.json")["credentials"]}
    assert material["id"] in known
    assert known[material["id"]] == material["secret"]
    assert material["slot"] == "prod"
    call = session.calls[-1]
    assert call.method == "POST"
    assert call.kwargs["json"] == {"slot": "prod"}


def test_smsgw_revoke_calls_the_retire_endpoint():
    session = FakeSession({"/sms/keys/sms_old_3/retire": [FakeResponse(200, sample("smsgw-retire.json"))]})
    provider = SmsGatewayProvider(
        "smsgw", {"endpoint": "https://smsgw.example.test", "secret": "svc"}, session=session
    )

    answer = provider.revoke(
        {"name": "smsgw.prod", "slot": "prod"}, "sms_old_3", keep_until="2026-11-25T10:00:12Z"
    )

    assert answer["ok"] is True
    call = session.calls[-1]
    assert call.method == "POST"
    assert call.url == "https://smsgw.example.test/sms/keys/sms_old_3/retire"
    assert "keep_until" in call.kwargs["json"]


def test_paygate_create_returns_the_minted_key():
    session = FakeSession({
        "/gateway/credentials": [
            FakeResponse(200, {"id": "pay_new_5", "secret": "pay-live-5", "created_at": "2026-11-25T10:00:00Z"})
        ]
    })
    provider = PaygateProvider(
        "paygate", {"endpoint": "https://paygate.example.test", "secret": "svc"}, session=session
    )

    material = provider.create({"name": "paygate.prod", "service": "paygate", "slot": "prod"})

    assert material["id"] == "pay_new_5"
    assert material["secret"] == "pay-live-5"


def test_provider_raises_on_a_server_error():
    from rotgraph.providers.base import ProviderError

    session = FakeSession({"/v1/keys": [FakeResponse(503, {"error": "later"})]})
    provider = SignerProvider(
        "signer", {"endpoint": "https://signer.example.test", "secret": "svc"}, session=session
    )

    try:
        provider.create({"name": "signer.prod", "slot": "prod"})
    except ProviderError as exc:
        assert "503" in str(exc)
    else:  # pragma: no cover
        raise AssertionError("expected ProviderError")
