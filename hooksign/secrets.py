"""密钥环：支持轮换期间同时存在多把 secret。"""

from __future__ import annotations

import time
from collections.abc import Iterable
from dataclasses import dataclass, field

from .errors import ConfigurationError


@dataclass(frozen=True)
class Secret:
    """一把 HMAC 密钥。

    参数：
        id:       给日志和监控用的名字，不参与签名计算。
        key:      原始密钥字节。
        not_after: 过期时间（unix 秒）。``None`` 表示不过期。
        created_at: 加入密钥环的时间（unix 秒），用于判断哪把更新。
    """

    id: str
    key: bytes
    not_after: float | None = None
    created_at: float = field(default_factory=time.time)


class SecretRing:
    """一组 secret，按加入顺序保存。

    轮换时的典型用法：先 ``add()`` 新 secret，旧 secret 设一个 ``not_after``，
    等下游都切过去之后旧 secret 自然过期。
    """

    def __init__(self, secrets: Iterable[Secret] = ()) -> None:
        self._secrets: list[Secret] = []
        for secret in secrets:
            self.add(secret)

    def add(self, secret: Secret) -> None:
        if not secret.key:
            raise ConfigurationError(f"secret {secret.id!r} 的 key 为空")
        if any(existing.id == secret.id for existing in self._secrets):
            raise ConfigurationError(f"secret id 重复：{secret.id!r}")
        self._secrets.append(secret)

    def active(self, *, now: float | None = None) -> list[Secret]:
        """所有尚未过期的 secret，**新的在前**。"""
        current = time.time() if now is None else float(now)
        alive = [
            secret
            for secret in self._secrets
            if secret.not_after is None or secret.not_after > current
        ]
        alive.sort(key=lambda secret: secret.created_at, reverse=True)
        return alive

    def primary(self) -> Secret:
        """最新的一把，用于签名出方向请求。"""
        alive = self.active()
        if not alive:
            raise ConfigurationError("密钥环里没有可用的 secret")
        return alive[0]

    def __len__(self) -> int:
        return len(self._secrets)
