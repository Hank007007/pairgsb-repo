# leaseledger

短期凭据租约服务的内核。调用方按 subject、scope 和 TTL 申请 lease；lease 可以续租、
主动吊销，也会随时间自然过期。策略按 scope 决定每个租约的最长有效期。

规范以 [`docs/lease-design.md`](docs/lease-design.md) 为准。

## 模块

| 文件 | 职责 |
|------|------|
| `leaseledger/clock.py` | `Clock` 协议、系统时钟与测试用固定时钟 |
| `leaseledger/scopes.py` | scope 文本解析与集合运算 |
| `leaseledger/policy.py` | 加载按 scope 配置的策略，给出 TTL 上限 |
| `leaseledger/store.py` | 租约表、查询、活跃判断、到期回收 |
| `leaseledger/issue.py` | 签发新租约 |
| `leaseledger/renew.py` | 从既有租约续租 |
| `leaseledger/revoke.py` | 主动吊销租约 |
| `leaseledger/records.py` | 审计记录的导入 / 导出 |
| `leaseledger/errors.py` | 异常类型 |

## 快速开始

```python
from datetime import datetime, timezone

from leaseledger import FixedClock, LeaseStore, Policy, issue, renew, revoke

clock = FixedClock(datetime(2026, 1, 1, 12, 0, tzinfo=timezone.utc))
store = LeaseStore(clock)
policy = Policy.load("config/policy.json")

lease = issue(
    store, policy, clock, subject="svc-orders", scopes="read,write", ttl_seconds=600
)
child = renew(store, policy, clock, lease.id, scopes="read", ttl_seconds=300)

print(store.is_active(child))

revoke(store, clock, lease.id, reason="key rotation")
```

## 运行测试

只用标准库和 pytest：

```bash
python -m pytest -q -p no:cacheprovider
```

（Python 3.10+）

## 审计记录

`leaseledger.records` 提供租约记录的 JSON 导入 / 导出，`examples/audit_replay.py`
演示把审计侧导出的记录重新装进租约表：

```bash
python examples/audit_replay.py examples/exported_leases.json
```
