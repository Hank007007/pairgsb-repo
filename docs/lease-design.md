# 租约服务设计说明

本文描述 `leaseledger` 必须满足的行为。文中使用「必须 / 不得」表述的条目都是规范性要求；
实现方式不在本文范围内，但对外可观察到的结果必须与本文一致。

- [§1 数据模型与状态](#1-数据模型与状态)
- [§2 续租：scope 只能收窄](#2-续租scope-只能收窄)
- [§3 吊销：沿派生链级联](#3-吊销沿派生链级联)
- [§4 时间与过期判定](#4-时间与过期判定)
- [§5 TTL 上限](#5-ttl-上限)
- [§6 scope 匹配](#6-scope-匹配)
- [§7 验收不变量](#7-验收不变量)

---

## §1 数据模型与状态

一条租约（lease）至少包含以下字段：

| 字段 | 类型 | 含义 |
|---|---|---|
| `id` | `str` | 租约标识，全局唯一 |
| `subject` | `str` | 租约持有者 |
| `scopes` | `tuple[str, ...]` | 该租约被授予的 scope 集合，元素为具体 scope |
| `issued_at` | `datetime` | 签发时刻 |
| `expires_at` | `datetime` | 到期时刻 |
| `parent_lease_id` | `str \| None` | 派生来源；签发出来的根租约为 `None` |
| `revoked_at` | `datetime \| None` | 首次被吊销的时刻；未吊销为 `None` |
| `revoked_reason` | `str \| None` | 吊销原因 |

租约在任何时刻处于且只处于下列三种状态之一：

1. **active**：未被吊销，且尚未到期；
2. **revoked**：`revoked_at` 非空；
3. **expired**：未被吊销，且已经到期。

`revoked` 与 `expired` 互不覆盖：一条已经过期的租约仍然可以被吊销并记录原因，
但吊销不会让它重新变为 active。只有 **active** 的租约可以用于鉴权，也只有 active
的租约可以作为续租的父租约。

scope 的文本格式：以 `:` 分隔的若干段，每段是以小写字母开头的
`[a-z0-9_-]` 串（例如 `read`、`read:admin`、`billing:invoice:export`）。
末尾的 `*` 只在 policy 规则中出现，表示整棵子树；签发给调用方的 scope 集合里
不得包含 `*`。

policy 由三部分组成：一组按 scope 配置的规则、可选的 `default` 规则、以及一个
全局 `max_ttl_seconds`。

---

## §2 续租：scope 只能收窄

续租（renew）不是原地修改，而是**从父租约派生一条新租约**。父租约本身保持不变，
其 `expires_at` 不因续租而顺延

规范要求：

1. 新租约的 `subject` 必须与父租约一致；`parent_lease_id` 等于父租约的 `id`。
2. 请求里给出的 scope 集合 **必须与父租约的 scope 集合求交集**：
   新租约的 scope 集合必须是父集合的子集，可以相等或更小。
3. 请求中出现而父租约没有的 scope **不得出现在新租约里**。实现可以选择
   把它静默裁掉，也可以整体拒绝本次续租；两种结果都合规，但绝不能授予父租约
   没有的权限。
4. 请求未给出 scope 时，沿用父租约的 scope 集合。
5. 交集为空时，续租必须被拒绝：不得签发一条没有任何 scope 的租约。
6. 父租约必须是 active；已吊销或已过期的父租约不得续租。
7. 新租约的 TTL 仍须满足 §5 的上限，不得因为「继承自父租约」而绕过上限。

示例（父租约 scopes = `("read",)`）：

| 续租请求 scopes | 允许的结果 |
|---|---|
| `read` | 新租约 scopes = `("read",)` |
| （不传） | 新租约 scopes = `("read",)` |
| `read,write` | 新租约 scopes = `("read",)`，或拒绝 |
| `write` | 拒绝（交集为空） |

---

## §3 吊销：沿派生链级联

吊销（revoke）以一条租约为起点，必须同时对**整棵派生子树**生效。

规范要求：

1. 吊销一条租约时，所有通过 `parent_lease_id` 直接或间接由它派生出来的租约
   **必须一并失效**，立即不可用于鉴权，无需等待各自到期。
2. 级联只向下，不向上：吊销子租约不影响父租约、祖父租约，也不影响兄弟子树。
3. 已被吊销的租约再次被吊销不得报错；其结果集合与第一次相同。
4. 每一次吊销都记录当时 `clock.now()` 作为 `revoked_at`，并记录传入的 `reason`。
   重复吊销不改变首次记录的时刻。
5. 吊销返回本次实际变为 revoked 的租约 ID 集合，至少包含起始租约，
   级联实现下还应包含所有新失效的后代。
6. 吊销不修改租约的 `expires_at`，也不删除记录；清理到期记录是到期回收的职责。

---

## §4 时间与过期判定

1. 所有时刻必须是 **UTC aware** 的 `datetime`（`tzinfo` 为 `timezone.utc` 或等价的
   UTC 偏移）。签发、续租、吊销、过期判断都必须使用注入的 `Clock` 提供的
   `now()`，不得在业务代码里直接读取系统时间。
2. 判断租约是否过期时，比较的双方必须是可比较的时间：不得出现一侧 aware、
   另一侧 naive 的直接比较。若外部输入可能是 naive 时间，必须按 UTC 解释后再比较。
3. **边界算过期**：租约在 `now >= expires_at` 时已经过期，只有在
   `now < expires_at` 时才算尚未到期。即 `expires_at == now` 的那一格是过期。
4. 活跃判定 = 未被吊销 **且** 未到期。吊销状态优先于时间判断。
5. 到期回收只清理已经过期且未被吊销的记录；已经吊销的记录保留用于审计。

---

## §5 TTL 上限

TTL 上限由 policy 按 scope 决定，不是全局一个数字。

1. 对一条即将签发的租约，其实际 TTL 必须同时满足：
   - 不超过 policy 全局 `max_ttl_seconds`；
   - 不超过 policy 为**该租约的每一个 scope** 给出的 `max_ttl_seconds`。
2. 一条租约带多个 scope 时，适用其中最严格（最小）的上限。
3. 请求的 TTL 超过适用上限时必须拒绝签发，不得静默按上限截断后再签发。
4. 适用范围同时包括首次签发（issue）和续租（renew）。
5. 某个 scope 在 policy 中没有精确或通配规则可匹配时，使用 `default` 规则；
   若 policy 没有配置 `default`，则该 scope 不可签发，必须拒绝。
6. 请求 TTL 必须为正整数秒；非正数、非整数，或超过全局上限的请求一律拒绝。

---

## §6 scope 匹配

policy 规则按 **scope 段** 做层级匹配，而不是按字串前缀：

1. 不含 `*` 的规则只匹配与它完全相同的 scope。
   `read` 规则只覆盖 `read` 本身，**不覆盖** `read:admin`、`read:admin:billing`
   或 `readonly`。
2. 以 `:*` 结尾的规则匹配该节点**之下**的整棵子树。例如 `billing:*` 覆盖
   `billing:invoice` 与 `billing:invoice:export`，但不覆盖 `billing` 本身，
   也不覆盖 `billingx`。
3. `*` 规则匹配任意 scope，优先级最低，通常作为 `default` 使用。
4. 多条规则同时匹配时，**取最长（段数最多）的那条**，与规则在文件中的先后顺序无关。
   例如 `read`（3600s）与 `read:admin`（300s）同时命中 `read:admin` 时，
   必须使用 `read:admin` 的 300s。
5. 没有任何规则匹配且没有 `default` 时，视为该 scope 无策略可用，必须拒绝签发。

---

## §7 验收不变量

无论实现如何组织，下列性质在任意时刻都必须成立：

1. **续租收窄**：对任意 `child = renew(parent, scopes=S)`，
   `set(child.scopes) ⊆ set(parent.scopes)`；父租约没有的 scope 不会出现在子租约上。
2. **吊销级联**：吊销任意租约后，其派生链上的每一条后代租约的活跃判定均为 `False`。
3. **过期边界**：在 aware 时间下调用活跃判定不得抛出 `TypeError`；
   `now == expires_at` 时判定为不活跃。
4. **TTL 上限**：每次签发/续租产生的租约，其 TTL 不超过 policy 对其任意 scope
   给出的上限，也不超过全局上限。
5. **策略层级**：`read:admin` 取到的是 `read:admin` 自己的规则，
   不会落到 `read` 的规则上。

---

## §8 公开接口

| 接口 | 说明 |
|---|---|
| `issue(store, policy, clock, *, subject, scopes, ttl_seconds, lease_id=None)` | 签发根租约 |
| `renew(store, policy, clock, lease_id, *, scopes=None, ttl_seconds=None, new_lease_id=None)` | 从父租约派生 |
| `revoke(store, clock, lease_id, *, reason=None)` | 吊销并返回受影响 ID |
| `LeaseStore.is_active(lease)` | 活跃判定 |
| `LeaseStore.purge_expired()` | 回收过期记录 |
| `Policy.lookup(scope)` / `Policy.max_ttl_for(scope)` | 规则查询 |
| `leaseledger.records.lease_from_record` / `lease_to_record` | 审计记录的导入 / 导出 |

错误类型见 `leaseledger/errors.py`：参数不合法用 `InvalidTTL` / `InvalidScope`，
超策略上限用 `TTLExceedsPolicy`，父租约不可续租用 `LeaseNotActive`，
ID 不存在用 `UnknownLease`，无可用策略用 `UnknownScope`。
