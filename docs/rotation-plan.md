# rotation-plan.md — 批次、顺序与配置

本文规定一个**轮换批次（batch）**怎样从配置读入、怎样判断它能不能执行，以及批次内
各个凭据（下称「key」）按什么顺序被处理。执行细节见 `swap-window.md`、`notify.md`、
`recovery.md`。

## §1 术语

- **key**：供应商签发的一把凭据，用 `<service>.<slot>` 标识，例如 `signer.prod`。
- **service**：`plan` 里 `services` 声明的供应商，对应 `rotgraph/providers/` 里的一个 adapter。
- **slot**：同一个 service 下的凭据名。
- **batch**：一次轮换请求覆盖的所有 key，由配置的 `batch` 字段给出。
- **dwell**：新旧两把 key 在供应商那边同时有效的时长（秒），定义见 `swap-window.md` §1.2。
- **消费者（consumer）**：内部要用这些 key 去调供应商的服务，每家在配置的 `consumers`
  里列出，轮换时通过 webhook 收到新 key 并回执，见 `notify.md`。

## §2 配置文件

### §2.1 结构

配置文件是 JSON，顶层对象包含：

| 字段 | 类型 | 含义 |
|---|---|---|
| `batch` | string | 批次名 |
| `dwell` | number | 本批双活时长，秒 |
| `services` | object | service → `{endpoint, secret}` |
| `keys` | array | 按书写顺序排列的 key 条目 |
| `consumers` | array | 消费者列表，每项 `{name, endpoint, secret}` |
| `notify` | object | 可选，见 §2.9 |

每个 key 条目：

```json
{
  "name": "signer.prod",
  "service": "signer",
  "slot": "prod",
  "depends_on": ["objstore.prod"],
  "current": {"id": "key_old_1", "secret": "..."}
}
```

`current` 描述轮换**之前**在用的那把 key；没有它说明这个 slot 是新建，不需要停旧。

### §2.2 依赖优先

`A depends_on B` 表示 A 的消费者流量会用到 B 签发的凭据，因此 **B 必须先于 A 完成轮换**。
一条边 `A depends_on B` 要求「B 已被处理完（新 key 建好并推送、回执齐全）」早于
「A 开始被处理」。

### §2.3 批次是整体

一个批次要么被完整接受并按顺序执行，要么在任何外部动作发生之前被整体拒绝。
不存在「先跑一半再报配置错误」这种对外可见的中间状态。

### §2.4 声明顺序

`keys` 数组的书写顺序是批次内的**主序**。任何时候在「依赖已经全部满足」的 key 中
选择下一个要处理的 key 时，必须选书写顺序最靠前的那个。实现不得基于名字、服务名、
哈希值或其它因素重新排序，也不得把互不依赖的 key 并发执行。

推论：如果一批 key 彼此都没有依赖，它们就严格按书写顺序执行。

### §2.5 批内唯一

`keys` 里的 `name` 必须两两不同；重复名字的配置是配置错误。

### §2.6 环检测

把 key 当作节点、`depends_on` 当作有向边构成的图必须是无环的（DAG）。只要图中存在
任何一个环（包括自指 `A depends_on A`），**整批拒绝**：一个 key 都不许动，供应商那边
一个新 key 都不许创建。

### §2.7 依赖必须可满足

每条依赖边必须指向本批次 `keys` 中存在的名字。指向不存在名字的 `depends_on`（拼写
错误、把已下线 key 的名字写了进来）属于配置错误。

### §2.8 配置错误一律整批拒绝

`depends_on` 指向本批次里不存在的名字，同样按配置错误处理，整批拒绝。

§2.5、§2.6、§2.7 的判定发生在**任何外部动作之前**。判定失败时：

1. 批次不得开始；
2. 不得调用任何 provider；
3. 不得写入任何表示「已开始」的 journal 记录；
4. 面向调用返回一个可以区分原因的错误（重复名字 / 环 / 未知依赖）。

### §2.9 notify 配置

`notify` 可选，字段与缺省值：

| 字段 | 缺省 | 含义 |
|---|---|---|
| `max_attempts` | 5 | 单个消费者单次推送的最大尝试次数（首发计入） |
| `backoff_base` | 1.0 | 指数退避的起始秒数 |
| `backoff_cap` | 8.0 | 单次退避的上限秒数 |
| `ack_timeout` | 60 | 等回执的总时长上限，秒 |
| `poll_interval` | 1.0 | 轮询回执的间隔，秒 |

## §3 顺序的正确性判据

一个执行顺序 `S` 是正确的，当且仅当：

1. `S` 是 `keys` 的一个排列；
2. 对每条 `A depends_on B`，`B` 在 `S` 中出现在 `A` 之前；
3. `S` 可以由 §2.4 的选择规则逐步产生：每一步从「所有依赖都已在 `S` 的前缀中」
   的 key 里挑书写顺序最靠前的那个。

`rotgraph/plan.py` 提供的三个接口：

- `load_plan(path)` → `Plan`，只做读入与结构校验（§2.1、§2.5）；
- `detect_cycle(plan)` → 环上的 key 名列表，或 `None`；只判定 §2.6；
- `order(plan)` → 一个满足 §3 的 key 名顺序列表。

`order()` 在配置本身不合法时（环、未知依赖、重复名字）必须抛错，不得返回一个
「尽量正确」的顺序。
