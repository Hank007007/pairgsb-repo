# leasekit

基于 Redis 的异步分布式租约库。

`leasekit` 允许客户端用 `lease_id` 抢占一个命名资源，并周期性续约（renew）；
租约到期未被续约则自动释放，其他客户端可以抢占。

## 起 Redis

需要一个可连的真实 Redis。

**方式一：本机已装 redis-server**（推荐）

```bash
service redis-server start     # Debian/Ubuntu
# macOS: brew services start redis
redis-cli ping                 # 期望返回 PONG
```

**方式二：docker compose**

```bash
docker compose up -d
```

默认连 `redis://localhost:6379/15`（最后一个 db 留给测试）。要连别的地方就设：

```bash
export LEASEKIT_REDIS_URL="redis://localhost:6379/15"
```

## 安装

```bash
python -m venv .venv
.venv/bin/python -m pip install -e ".[dev]"       # Linux / macOS / WSL
# .venv/Scripts/python -m pip install -e ".[dev]" # Windows 原生
```

## 用法

```python
from leasekit import LeaseManager
from leasekit.backend.redis import RedisBackend

backend = RedisBackend(url="redis://localhost:6379/0")
mgr = LeaseManager(backend, default_ttl_ms=5000)

lease = await mgr.acquire("orders:42", ttl_ms=5000)
try:
    ...  # 独占 orders:42
finally:
    await mgr.release(lease.lease_id)
```

`acquire` 在资源已被占用时抛 `LeaseNotHeld`；`renew` 在租约已过期或被他人接管时
抛 `LeaseExpired`。

## 测试

```bash
# 先确认 Redis 在跑
redis-cli ping

.venv/bin/python -m pytest -q
```

测试直接跑在真实 Redis 上，用 db 15，每个用例前后 `FLUSHDB`。

## 模块

| 路径 | 作用 |
|---|---|
| `leasekit/manager.py` | `Lease` 与 `LeaseManager`，全部公开 API |
| `leasekit/backend/base.py` | `Backend` 协议，`LeaseManager` 依赖的最小接口 |
| `leasekit/backend/redis.py` | `RedisBackend`，基于 `redis.asyncio` |
| `leasekit/errors.py` | `LeaseError` / `LeaseNotHeld` / `LeaseExpired` |

## 设计说明

- 租约在 Redis 里存成 `leasekit:lease:<resource>` → `owner` 一个键值对，
  过期时间由 Redis 的 `PX` 负责，不依赖客户端时钟。
- `LeaseManager` 内部缓存本实例持有过的租约，因此同一个 `LeaseManager` 实例上，
  `acquire` 的返回值和 `renew` / `release` 的入参始终一致。
- `Backend` 是协议而非基类，接别的存储只需实现同样几个方法。

## 变更记录

见 [CHANGELOG.md](CHANGELOG.md)。
