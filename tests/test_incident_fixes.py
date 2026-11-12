"""回归测试：逐条覆盖 docs/incident-reports.md 记录的问题单。

每个用例都以 docs/signature-v2.md 为基准构造，不依赖被测实现自洽。
"""

from __future__ import annotations

import hashlib
import hmac
from concurrent.futures import ThreadPoolExecutor

import pytest

from hooksign import Secret, SecretRing, WebhookVerifier
from hooksign.cache import ReplayCache
from hooksign.canonical import canonical_string
from hooksign.signing import sign_header
from hooksign.store import DeliveryStore
from hooksign.verify import SignedRequest

NOW = 1_700_000_000
KEY = b"whsec_test_primary"
PATH = "/webhooks/orders"


def spec_signature(key: bytes, method: str, path: str, timestamp: int, body: bytes) -> str:
    """严格按 docs/signature-v2.md 第 2、3 节手工计算签名。"""
    message = "\n".join(
        [
            "v2",
            method.upper(),
            path,
            str(timestamp),
            hashlib.sha256(body).hexdigest(),
        ]
    )
    return hmac.new(key, message.encode("utf-8"), hashlib.sha256).hexdigest()


def make_verifier(*secrets: Secret) -> WebhookVerifier:
    ring = SecretRing(list(secrets))
    return WebhookVerifier(
        ring,
        tolerance_s=300,
        replay_cache=ReplayCache(ttl_s=300),
        clock=lambda: NOW,
    )


def make_request(
    body: bytes,
    *,
    key: bytes = KEY,
    timestamp: int = NOW,
    header_name: str = "X-Signature",
) -> SignedRequest:
    signature = spec_signature(key, "POST", PATH, timestamp, body)
    return SignedRequest(
        method="POST",
        path=PATH,
        headers={header_name: f"t={timestamp},v1={signature}"},
        body=body,
    )


# ---------------------------------------------------------------------
# 2026-10-28 合作方 C：对端按规范实现，签名始终对不上
# ---------------------------------------------------------------------
def test_canonical_string_has_exactly_five_lines():
    digest = hashlib.sha256(b"{}").hexdigest()

    message = canonical_string(method="post", path=PATH, timestamp=NOW, digest=digest)

    assert message == f"v2\nPOST\n{PATH}\n{NOW}\n{digest}"


def test_spec_compliant_external_signature_accepted():
    verifier = make_verifier(Secret(id="primary", key=KEY))
    body = b'{"id": "evt_c"}'

    result = verifier.verify(make_request(body))

    assert result.ok is True
    assert result.secret_id == "primary"


# ---------------------------------------------------------------------
# 2026-10-31 安全评审：签名比较必须是常量时间
# ---------------------------------------------------------------------
def test_signature_compared_in_constant_time(monkeypatch):
    calls = []
    original = hmac.compare_digest

    def spy(a, b):
        calls.append((a, b))
        return original(a, b)

    monkeypatch.setattr(hmac, "compare_digest", spy)

    verifier = make_verifier(Secret(id="primary", key=KEY))
    assert verifier.verify(make_request(b"{}")).ok is True
    assert calls, "verify() 没有使用 hmac.compare_digest 做签名比较"


# ---------------------------------------------------------------------
# 2026-11-03 内部：重复投递偶发双执行 / :memory: 报 no such table
# ---------------------------------------------------------------------
def test_delivery_store_in_memory_works():
    store = DeliveryStore(":memory:")

    assert store.record("evt_mem") is True
    assert store.record("evt_mem") is False
    assert store.seen("evt_mem") is True


def test_delivery_store_concurrent_record_only_succeeds_once(tmp_path):
    store = DeliveryStore(str(tmp_path / "deliveries.db"))

    with ThreadPoolExecutor(max_workers=8) as pool:
        results = list(pool.map(lambda _: store.record("evt_dup"), range(32)))

    assert results.count(True) == 1
    assert results.count(False) == 31


# ---------------------------------------------------------------------
# 2026-11-05 内部：同一秒的不同事件被误判 replay
# ---------------------------------------------------------------------
def test_same_timestamp_different_events_are_not_replay():
    verifier = make_verifier(Secret(id="primary", key=KEY))

    first = verifier.verify(make_request(b'{"id": "evt_1"}', timestamp=NOW))
    second = verifier.verify(make_request(b'{"id": "evt_2"}', timestamp=NOW))

    assert first.ok is True
    assert second.ok is True


def test_identical_signature_twice_is_replay():
    verifier = make_verifier(Secret(id="primary", key=KEY))
    request = make_request(b'{"id": "evt_1"}', timestamp=NOW)

    assert verifier.verify(request).ok is True
    replay = verifier.verify(request)
    assert replay.ok is False
    assert replay.reason == "replay"


def test_replay_cache_concurrent_check_only_succeeds_once():
    cache = ReplayCache(ttl_s=60)

    with ThreadPoolExecutor(max_workers=8) as pool:
        results = list(pool.map(lambda _: cache.check_and_store("k"), range(32)))

    assert results.count(True) == 1


# ---------------------------------------------------------------------
# 2026-11-07 合作方 A：非 UTF-8 正文（PDF）全部被拒
# ---------------------------------------------------------------------
def test_non_utf8_body_roundtrip():
    verifier = make_verifier(Secret(id="primary", key=KEY))
    body = b"%PDF-1.4\n%\xe2\xe3\xcf\xd3\nstream\n\x89\xfe\xff\x00encrypted\nendstream"

    result = verifier.verify(make_request(body))

    assert result.ok is True


def test_non_utf8_body_signed_by_sign_header():
    verifier = make_verifier(Secret(id="primary", key=KEY))
    body = b"\x00\x01\xfe\xff not valid utf-8"
    headers = sign_header(
        Secret(id="primary", key=KEY), method="POST", path=PATH, body=body, timestamp=NOW
    )

    result = verifier.verify(
        SignedRequest(method="POST", path=PATH, headers=headers, body=body)
    )

    assert result.ok is True


# ---------------------------------------------------------------------
# 2026-11-09 合作方 B：多密钥轮换期 401 / 头名被网关归一化成大写
# ---------------------------------------------------------------------
@pytest.mark.parametrize("header_name", ["X-SIGNATURE", "x-signature", "X-Signature"])
def test_header_name_is_case_insensitive(header_name):
    verifier = make_verifier(Secret(id="primary", key=KEY))

    result = verifier.verify(make_request(b"{}", header_name=header_name))

    assert result.ok is True


def test_all_active_secrets_are_tried_during_rotation():
    verifier = make_verifier(
        Secret(id="old", key=b"whsec_old"),
        Secret(id="new", key=b"whsec_new"),
    )

    by_old = verifier.verify(make_request(b'{"id": "a"}', key=b"whsec_old"))
    by_new = verifier.verify(make_request(b'{"id": "b"}', key=b"whsec_new"))

    assert by_old.ok is True
    assert by_old.secret_id == "old"
    assert by_new.ok is True
    assert by_new.secret_id == "new"


def test_expired_secret_is_not_used():
    verifier = make_verifier(
        Secret(id="retired", key=b"whsec_old", not_after=NOW - 1),
        Secret(id="current", key=KEY),
    )

    result = verifier.verify(make_request(b"{}", key=b"whsec_old"))

    assert result.reason == "bad_signature"


def test_any_matching_v1_passes():
    verifier = make_verifier(Secret(id="primary", key=KEY))
    body = b"{}"
    good = spec_signature(KEY, "POST", PATH, NOW, body)
    request = SignedRequest(
        method="POST",
        path=PATH,
        headers={"X-Signature": f"t={NOW},v1=deadbeef,v1={good}"},
        body=body,
    )

    assert verifier.verify(request).ok is True


# ---------------------------------------------------------------------
# 规范第 5 节：出方向签名用最后添加且未过期的那把
# ---------------------------------------------------------------------
def test_primary_is_last_added_unexpired_secret():
    ring = SecretRing()
    ring.add(Secret(id="first", key=b"a", created_at=2000.0))
    ring.add(Secret(id="second", key=b"b", created_at=1000.0))

    assert ring.primary().id == "second"

    ring.add(Secret(id="expired", key=b"c", not_after=1.0))
    assert ring.primary().id == "second"
