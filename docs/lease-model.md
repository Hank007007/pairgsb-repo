# Lease 模型与时间语义

本文是 leasefold 的行为规范。`src/credlease/` 下的实现以本文为准；当实现与本文不一致时，以本文描述的行为为期望行为。

## 1. 角色与策略

业务方不直接持有数据库账号，而是按**角色**（role）向 broker 申请租约（lease）。每个角色有一条策略（policy），由 `policy.py` 从配置文件加载：

```json
{
  "roles": {
    "analyst": {
      "backend": "db-analyst",
      "max_ttl": "15m",
      "soft_ttl": "5m",
      "reuse": true
    }
  }
}
```

| 字段 | 含义 |
| --- | --- |
| `backend` | 该角色对应的后端凭据名，转交给 `backend.py` 的驱动签发 |
| `max_ttl` | 该策略允许的**有效期上限**，任何签发出的 lease 的有效期都不得超过它 |
| `soft_ttl` | 软 TTL，客户端主动续租的建议节奏；可小于 `max_ttl` |
| `reuse` | 是否允许同一角色在有效期内复用已有 lease |

时长字段接受整数秒或 `"15m"` / `"2h"` / `"30s"` 形式的字符串。

### 1.1 有效期的解析与截断

调用方可以通过两种方式指定期望有效期：

- `ttl_seconds`：整数，单位为秒；
- `ttl`：数字，单位为**分钟**。

两者都未提供时，使用后端默认有效期。无论使用哪个字段、是否提供，最终签发的有效期都必须满足：

```
0 < effective_ttl <= policy.max_ttl
```

超过 `max_ttl` 的请求应当被**截断**到 `max_ttl`，而不是被忽略、也不是直接报错。两个字段同时出现时，`ttl_seconds` 优先。非正数属于非法输入，应被拒绝。

## 2. Lease 状态

一条 lease 从签发到失效，只会经历下表的状态迁移。

| 状态 | 含义 | 可迁往 |
| --- | --- | --- |
| `issued` | 已向后端申请到凭据，尚未被使用过 | `active`、`revoked`、`expired` |
| `active` | 已交付给客户端且处于有效期内 | `renewed`、`revoked`、`expired` |
| `renewed` | 成功续租，有效期被延长 | `renewed`、`revoked`、`expired` |
| `revoked` | 已撤销，凭据在后端失效 | 终态 |
| `expired` | 已到期，凭据不可再用 | 终态 |

约定：

- 只有 `issued`、`active`、`renewed` 被视为**有效（live）**态；`revoked` 与 `expired` 是终态，不能再迁移。
- 一条 lease 处于终态后，再次对其执行续租或撤销都不得改变其状态，也不得报错（幂等）。
- 撤销、到期都会让后端凭据失效；不同状态之间的迁移必须记录发生时间，便于审计。

## 3. 时间语义

每条 lease 记录三个时间点：

| 字段 | 含义 |
| --- | --- |
| `issued_at` | 签发时刻 |
| `expires_at` | 当前有效期的结束时刻（滑动窗口） |
| `absolute_expires_at` | 绝对到期时刻，签发时确定，永不推迟 |

时间统一从 `clock.py` 注入的时钟读取，测试中可替换。

- **软 TTL 与续租**：续租成功时，`expires_at` 从续租时刻起顺延一个 `soft_ttl`（滑动窗口），可以多次续租。
- **绝对到期是硬线**：`absolute_expires_at` 在签发时确定，等于签发时刻加 `max_ttl`。续租只能在它之前发生；一次续租后的 `expires_at` 不得晚于 `absolute_expires_at`。若续租请求发生时刻已经到达或超过 `absolute_expires_at`，续租必须被**拒绝**，lease 转为 `expired`。
- **到期判定**：在任意时刻读取 lease 时，若当前时刻达到或超过 `expires_at` 或 `absolute_expires_at`，该 lease 视为 `expired`，且不能再次变成有效状态。
- 到期是一个惰性判定：读取时发现已过期，就应当把状态固定为 `expired`。

## 4. 撤销

撤销以 `(subject, role)` 为作用域：

1. 该 subject 在该 role 下的**全部仍然有效的 lease**（`issued` / `active` / `renewed`）都必须变为 `revoked`。
2. 撤销必须通知后端，让对应凭据立即失效；后端报告失败时，`revoke` 必须向上抛出错误或返回失败，不能报告成功。
3. 撤销是**幂等**的：对同一 `(subject, role)` 重复撤销不报错、不产生额外副作用；对不存在的 subject 撤销同样不报错。
4. 撤销必须**同步失效缓存**：撤销完成后，紧接着为该 subject/role 发起的请求不得再返回已撤销的凭据，而应重新向后端申请。

## 5. 缓存

为了减少后端压力，broker 对每个角色维护一层本地缓存。

- 缓存键必须能唯一标识“在什么策略下为谁签发的凭据”，至少包含角色、subject 和当前生效的策略版本。
- 缓存条目只在 lease 仍有效、且策略版本未变化时可被复用。
- **策略版本**：`Policy.version` 在每次热加载时自增。版本一旦变化，此前缓存的条目全部视为失效；下一次请求必须重新访问后端，不能命中旧缓存。
- 当策略的 `reuse` 为 false 时，不得复用缓存中的凭据，每次重新签发。
- 撤销会显式清除受影响的缓存条目（见 §4.4）；租约自然到期时该条目也不得被复用。

## 6. broker 对外接口

`Broker` 暴露三个方法：

| 方法 | 行为 |
| --- | --- |
| `request_lease(subject, role, *, ttl=None, ttl_seconds=None)` | 申请 lease；命中可复用缓存则直接返回，否则向后端签发并写入缓存 |
| `renew_lease(lease_id)` | 续租；遵守 §3 的滑动窗口与绝对到期规则 |
| `revoke(subject, role)` | 按 §4 撤销 |

所有方法对非法输入（未知角色、未知 lease、非法时长）应给出明确错误，而不是静默返回一个看似成功的结果。

## 7. 后端接口

`backend.py` 的 `CredentialBackend` 只定义协议，具体实现（内存或真实数据库）可替换：

| 方法 | 约定 |
| --- | --- |
| `issue(subject, role, backend, ttl_seconds)` | 返回一份凭据；失败抛 `BackendError` |
| `revoke(credential_id)` | 撤销后端凭据；失败抛 `BackendError` |

broker 不得吞掉后端的失败：签发失败不能产出 lease，撤销失败不能报告撤销成功。
