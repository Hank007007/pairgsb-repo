"""线上工单回归用例：每条对应 docs/incident-reports.md 中的一条记录。

根因以 docs/signature-v2.md 为准；这些用例锁定修复后的规范行为。
"""

from __future__ import annotations

import hashlib
import hmac
import json
import threading
import time
from unittest import mock

import pytest

import hooksign.verify
from hooksign import Secret, SecretRing, WebhookVerifier
from hooksign.cache import ReplayCache
from hooksign.canonical import body_digest, canonical_string
from hooksign.signing import sign_header
from hooksign.store import DeliveryStore
from hooksign.verify import SignedRequest

SECRET_TEXT = "whsec_incident"


def make_verifier(*secrets: Secret) -> WebhookVerifier:
    return WebhookVerifier(
        SecretRing(secrets), replay_cache=ReplayCache(ttl_s=300)
    )


def make_request(
    secret: Secret,
    body: bytes,
    *,
    path: str = "/webhooks/orders",
    timestamp: int | None = None,
    header_name: str = "X-Signature",
) -> SignedRequest:
    headers = sign_header(
        secret, method="POST", path=path, body=body, timestamp=timestamp
    )
    return SignedRequest(
        method="POST",
        path=path,
        headers={header_name: headers["X-Signature"]},
        body=body,
    )


# 2026-11-07 合作方 A：PDF（非合法 UTF-8）全部被拒
def test_incident_a_binary_body_verifies():
    secret = Secret(id="primary", key=SECRET_TEXT.encode())
    body = b"%PDF-1.7\n" + bytes(range(256)) + b"\x9f\xf0\xff\xfe encrypted"
    with pytest.raises(UnicodeDecodeError):
        body.decode("utf-8")

    result = make_verifier(secret).verify(make_request(secret, body))

    assert result.ok, result.reason
    assert result.reason == ""


# 2026-11-09 合作方 B：网关把头名归一化成 X-SIGNATURE
def test_incident_b_header_name_is_case_insensitive():
    secret = Secret(id="primary", key=SECRET_TEXT.encode())
    body = b'{"id": "evt_b1"}'

    for header_name in ("X-Signature", "X-SIGNATURE", "x-signature", "x-SiGnAtUrE"):
        verifier = make_verifier(secret)
        result = verifier.verify(
            make_request(secret, body, header_name=header_name)
        )
        assert result.ok, f"{header_name}: {result.reason}"


# 2026-11-09 合作方 B：轮换期间新旧密钥混用签名
def test_incident_b_all_active_secrets_are_tried():
    old = Secret(id="old", key=b"whsec_old", created_at=1000.0)
    new = Secret(id="new", key=b"whsec_new", created_at=2000.0)
    verifier = make_verifier(old, new)
    body = b'{"id": "evt_b2"}'

    result_old = verifier.verify(make_request(old, body))
    result_new = verifier.verify(make_request(new, body))

    assert result_old.ok and result_old.secret_id == "old"
    assert result_new.ok and result_new.secret_id == "new"


# 2026-10-28 合作方 C：对端（Go）按规范算出的签名始终对不上
def test_incident_c_canonical_string_has_exactly_five_lines():
    digest = body_digest(b'{"id": "evt_c"}')

    message = canonical_string(
        method="post", path="/webhooks/orders", timestamp=1700000000, digest=digest
    )

    assert message == "v2\nPOST\n/webhooks/orders\n1700000000\n" + digest
    assert not message.endswith("\n")


def test_incident_c_signature_matches_reference_computation():
    # 模拟对端实现：完全按规范文档手算，不经过 hooksign 的任何辅助函数
    key = b"whsec_go_side"
    body = b'{"id": "evt_c"}'
    timestamp = 1700000000
    digest = hashlib.sha256(body).hexdigest()
    message = f"v2\nPOST\n/webhooks/orders\n{timestamp}\n{digest}".encode("utf-8")
    expected = hmac.new(key, message, hashlib.sha256).hexdigest()

    secret = Secret(id="primary", key=key)
    headers = sign_header(
        secret, method="POST", path="/webhooks/orders", body=body, timestamp=timestamp
    )

    assert headers["X-Signature"] == f"t={timestamp},v1={expected}"


# 2026-11-05 内部：同一秒内 300 条不同事件被误判 replay
def test_incident_same_second_distinct_events_are_not_replay():
    secret = Secret(id="primary", key=SECRET_TEXT.encode())
    verifier = make_verifier(secret)
    moment = int(time.time())

    for index in range(300):
        body = json.dumps({"id": f"evt_burst_{index}"}).encode()
        result = verifier.verify(make_request(secret, body, timestamp=moment))
        assert result.ok and result.reason == "", f"evt_burst_{index}: {result.reason}"


def test_incident_same_signature_twice_is_idempotent_replay():
    secret = Secret(id="primary", key=SECRET_TEXT.encode())
    verifier = make_verifier(secret)
    request = make_request(secret, b'{"id": "evt_dup"}')

    first = verifier.verify(request)
    second = verifier.verify(request)

    assert first.ok and first.reason == ""
    assert second.ok and second.reason == "replay"


# 2026-11-03 内部：DeliveryStore(":memory:") 报 no such table
def test_incident_delivery_store_works_in_memory():
    store = DeliveryStore(":memory:")

    assert store.record("evt_a") is True
    assert store.record("evt_a") is False
    assert store.seen("evt_a") is True
    assert store.seen("evt_b") is False


# 2026-11-03 内部：并发重复投递双执行 + sqlite3.IntegrityError
def test_incident_delivery_store_concurrent_record_only_succeeds_once():
    store = DeliveryStore(":memory:")
    results: list[bool] = []
    errors: list[Exception] = []
    barrier = threading.Barrier(8)

    def worker() -> None:
        try:
            barrier.wait(timeout=5)
            results.append(store.record("evt_race"))
        except Exception as exc:  # 要断言没有任何异常
            errors.append(exc)

    threads = [threading.Thread(target=worker) for _ in range(8)]
    for thread in threads:
        thread.start()
    for thread in threads:
        thread.join()

    assert errors == []
    assert results.count(True) == 1
    assert results.count(False) == 7


# 2026-10-31 安全评审：签名比较必须是常量时间
def test_incident_signature_compare_uses_compare_digest():
    secret = Secret(id="primary", key=SECRET_TEXT.encode())
    verifier = make_verifier(secret)
    request = make_request(secret, b'{"id": "evt_timing"}')

    with mock.patch.object(
        hooksign.verify.hmac, "compare_digest", wraps=hmac.compare_digest
    ) as spy:
        result = verifier.verify(request)

    assert result.ok, result.reason
    assert spy.called
