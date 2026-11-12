"""基线测试：只覆盖主路径。"""

from __future__ import annotations

import json
import time

import pytest

from hooksign import Secret, SecretRing, WebhookVerifier, parse_signature_header
from hooksign.cache import ReplayCache
from hooksign.signing import sign_header
from hooksign.store import DeliveryStore
from hooksign.verify import SignedRequest

SECRET_TEXT = "whsec_test_primary"


@pytest.fixture()
def ring() -> SecretRing:
    return SecretRing([Secret(id="primary", key=SECRET_TEXT.encode())])


@pytest.fixture()
def verifier(ring) -> WebhookVerifier:
    return WebhookVerifier(ring, tolerance_s=300, replay_cache=ReplayCache(ttl_s=300))


def make_request(body: bytes, *, method: str = "POST", path: str = "/webhooks/orders"):
    secret = Secret(id="primary", key=SECRET_TEXT.encode())
    headers = sign_header(secret, method=method, path=path, body=body)
    headers["Content-Type"] = "application/json"
    return SignedRequest(method=method, path=path, headers=headers, body=body)


def test_parse_header_basic():
    parsed = parse_signature_header("t=1700000000,v1=abc")

    assert parsed.timestamp == 1700000000
    assert parsed.signatures == ("abc",)


def test_parse_header_multiple_signatures():
    parsed = parse_signature_header("t=1700000000,v1=aaa, v1=bbb")

    assert parsed.signatures == ("aaa", "bbb")


def test_roundtrip_json_body(verifier):
    body = json.dumps({"id": "evt_1"}).encode()

    result = verifier.verify(make_request(body))

    assert result.ok is True
    assert result.reason == ""
    assert result.secret_id == "primary"


def test_tampered_body_rejected(verifier):
    request = make_request(b'{"amount": 100}')
    tampered = SignedRequest(
        method=request.method,
        path=request.path,
        headers=request.headers,
        body=b'{"amount": 999}',
    )

    assert verifier.verify(tampered).reason == "bad_signature"


def test_missing_header_rejected(verifier):
    request = SignedRequest(method="POST", path="/x", headers={}, body=b"{}")

    assert verifier.verify(request).reason == "missing_header"


def test_stale_timestamp_rejected(verifier):
    body = b'{"id": "evt_2"}'
    secret = Secret(id="primary", key=SECRET_TEXT.encode())
    headers = sign_header(
        secret,
        method="POST",
        path="/webhooks/orders",
        body=body,
        timestamp=int(time.time()) - 3600,
    )

    request = SignedRequest(
        method="POST", path="/webhooks/orders", headers=headers, body=body
    )
    assert verifier.verify(request).reason == "timestamp_skew"


def test_replay_cache_detects_duplicate():
    cache = ReplayCache(ttl_s=60)

    assert cache.check_and_store("k1") is True
    assert cache.check_and_store("k1") is False


def test_delivery_store_records_once(tmp_path):
    store = DeliveryStore(str(tmp_path / "deliveries.db"))

    assert store.record("evt_a") is True
    assert store.record("evt_a") is False
    assert store.seen("evt_a") is True
    assert store.seen("evt_b") is False


def test_secret_ring_primary_is_newest():
    ring = SecretRing(
        [
            Secret(id="old", key=b"old", created_at=1000.0),
            Secret(id="new", key=b"new", created_at=2000.0),
        ]
    )

    assert ring.primary().id == "new"
    assert [s.id for s in ring.active(now=2500.0)] == ["new", "old"]
