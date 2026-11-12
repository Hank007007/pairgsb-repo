"""构造待签名字符串（v2）。"""

from __future__ import annotations

import hashlib

VERSION = "v2"


def body_digest(body: bytes) -> str:
    """body 原始字节的 SHA-256，小写十六进制。"""
    return hashlib.sha256(body).hexdigest()


def canonical_string(*, method: str, path: str, timestamp: int, digest: str) -> str:
    """按 v2 规范拼出待签名字符串。

    各字段之间用 ``\\n`` 连接，结尾不带换行。
    """
    return "\n".join(
        [
            VERSION,
            method.upper(),
            path,
            str(timestamp),
            digest,
        ]
    )
