"""hooksign 的异常类型。"""

from __future__ import annotations


class HookSignError(Exception):
    """hooksign 的基类异常。"""


class SignatureVerificationError(HookSignError):
    """签名校验没通过。

    参数：
        reason: 机器可读的原因码，见 ``WebhookVerifier`` 的 ``VerificationResult.reason``。
        timestamp: 请求头里解析出的时间戳（没有就是 ``None``）。
    """

    def __init__(self, reason: str, *, timestamp: int | None = None) -> None:
        super().__init__(reason)
        self.reason = reason
        self.timestamp = timestamp


class ConfigurationError(HookSignError):
    """SecretRing / 校验器的配置不合法。"""
