"""出方向签名：给请求生成 ``X-Signature`` 头。"""

from __future__ import annotations

import hashlib
import hmac
import time

from .canonical import body_digest, canonical_string
from .headers import HEADER_NAME
from .secrets import Secret


def sign(
    secret: Secret,
    *,
    method: str,
    path: str,
    body: bytes,
    timestamp: int | None = None,
) -> str:
    """算出签名值（不含 ``t=`` 前缀）。"""
    moment = int(time.time()) if timestamp is None else int(timestamp)
    digest = body_digest(body)
    message = canonical_string(
        method=method, path=path, timestamp=moment, digest=digest
    )
    return hmac.new(secret.key, message.encode("utf-8"), hashlib.sha256).hexdigest()


def sign_header(
    secret: Secret,
    *,
    method: str,
    path: str,
    body: bytes,
    timestamp: int | None = None,
) -> dict[str, str]:
    """返回可以直接塞进请求头。"""
    moment = int(time.time()) if timestamp is None else int(timestamp)
    signature = sign(secret, method=method, path=path, body=body, timestamp=moment)
    return {HEADER_NAME: f"t={moment},v1={signature}"}
