"""入方向校验：验证回调请求的签名。"""

from __future__ import annotations

import hashlib
import hmac
import time
from collections.abc import Mapping
from dataclasses import dataclass

from .cache import ReplayCache
from .canonical import body_digest, canonical_string
from .headers import HEADER_NAME, parse_signature_header
from .secrets import SecretRing


@dataclass(frozen=True)
class SignedRequest:
    """待校验的一次回调请求。"""

    method: str
    path: str
    headers: Mapping[str, str]
    body: bytes


@dataclass(frozen=True)
class VerificationResult:
    """校验结果。

    ``reason`` 取值：

    - ``""``                 通过
    - ``missing_header``     没有 ``X-Signature`` 头
    - ``malformed_header``   头式不对，或没有可用的 ``v1``
    - ``timestamp_skew``     时间戳超出容差
    - ``bad_signature``      签名不匹配
    - ``replay``             时间窗内重复投递
    """

    ok: bool
    reason: str = ""
    secret_id: str | None = None
    timestamp: int | None = None


class WebhookVerifier:
    """按 v2 规范校验回调签名。

    参数：
        secrets:      密钥环。轮换期间可以同时放多把。
        tolerance_s:  允许的时间偏差，单位秒。默认 300。
        replay_cache: 防重放缓存；传 ``None`` 表示不做防重放。
        clock:        取当前时间的函数，单测时可以注入。
    """

    def __init__(
        self,
        secrets: SecretRing,
        *,
        tolerance_s: float = 300.0,
        replay_cache: ReplayCache | None = None,
        clock=time.time,
    ) -> None:
        self._secrets = secrets
        self._tolerance_s = float(tolerance_s)
        self._replay_cache = replay_cache
        self._clock = clock

    def verify(self, request: SignedRequest) -> VerificationResult:
        raw = request.headers.get(HEADER_NAME) or request.headers.get(
            HEADER_NAME.lower()
        )
        if not raw:
            return VerificationResult(ok=False, reason="missing_header")

        try:
            parsed = parse_signature_header(raw)
        except ValueError:
            return VerificationResult(ok=False, reason="malformed_header", timestamp=None)

        timestamp = parsed.timestamp
        if timestamp is None or not parsed.signatures:
            return VerificationResult(ok=False, reason="malformed_header")

        now = self._clock()
        if abs(now - timestamp) > self._tolerance_s:
            return VerificationResult(
                ok=False, reason="timestamp_skew", timestamp=timestamp
            )

        # 服务端拿到的是解码后的正文，签名按文本重新算一遍
        body_text = request.body.decode("utf-8", errors="replace")
        digest = body_digest(body_text.encode("utf-8"))

        secret = self._secrets.primary()
        message = canonical_string(
            method=request.method,
            path=request.path,
            timestamp=timestamp,
            digest=digest,
        )
        expected = hmac.new(
            secret.key, message.encode("utf-8"), hashlib.sha256
        ).hexdigest()

        matched = None
        for candidate in parsed.signatures:
            if expected == candidate:
                matched = secret
                break

        if matched is None:
            return VerificationResult(
                ok=False, reason="bad_signature", timestamp=timestamp
            )

        if self._replay_cache is not None:
            if not self._replay_cache.check_and_store(str(timestamp), now=now):
                return VerificationResult(
                    ok=False,
                    reason="replay",
                    secret_id=matched.id,
                    timestamp=timestamp,
                )

        return VerificationResult(
            ok=True, reason="", secret_id=matched.id, timestamp=timestamp
        )
