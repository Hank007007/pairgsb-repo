"""LeaseManager 依赖的最小后端接口。

只需要实现下面这几个方法，就可以作为 LeaseManager 的存储后端
（生产环境用 Redis；接别的存储只需实现同样几个方法）。
"""

from __future__ import annotations

from typing import Protocol


class Backend(Protocol):
    """键值后端。所有方法都是异步的。"""

    async def get(self, key: str) -> str | None:
        """读取 key，不存在返回 None。"""
        ...

    async def set(self, key: str, value: str, ttl_ms: int) -> None:
        """无条件写入 key，并设置毫秒级过期时间。"""
        ...

    async def delete(self, *keys: str) -> None:
        """删除若干 key，忽略不存在的。"""
        ...

    async def expire(self, key: str, ttl_ms: int) -> None:
        """把 key 的过期时间重置为 ttl_ms。key 不存在时什么都不做。"""
        ...

    async def close(self) -> None:
        """释放底层连接。"""
        ...
