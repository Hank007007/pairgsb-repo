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

    # 下面三个是**可选**的原子操作：后端不实现时，LeaseManager 会
    # 退回到 get + set/expire/delete 的组合。但旧组合不是原子的，
    # 并发下会出现两个客户端同时持有同一资源，因此支持竞态语义的后端
    # （如 Redis）应当实现它们。

    async def set_if_absent(self, key: str, value: str, ttl_ms: int) -> bool:
        """仅当 key 不存在时写入并设置过期时间；已存在则不改动。

        返回是否写入成功。整个操作必须是原子的。
        """
        ...

    async def expire_if_owner(self, key: str, holder: str, ttl_ms: int) -> bool:
        """仅当 key 当前值等于 holder 时才重置过期时间。

        返回是否续期成功（key 不存在或已被他人接管时为 False）。
        「比较持有者 + 续期」必须是原子的。
        """
        ...

    async def delete_if_owner(self, key: str, holder: str) -> bool:
        """仅当 key 当前值等于 holder 时才删除。

        返回是否删除成功（key 不存在或已被他人接管时为 False）。
        「比较持有者 + 删除」必须是原子的。
        """
        ...

    async def close(self) -> None:
        """释放底层连接。"""
        ...
