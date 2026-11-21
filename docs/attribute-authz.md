# Attribute Authorization — scimfence

本文规定 token、租户绑定与属性级读写授权。§引用与 scim-profile.md 对应。

## §1 Token 与调用方身份

### §1.1 载体

上游以

```
Authorization: Bearer <token>
```

调用。token 由 `scimfence` 的签发方离线签发，内容对调用方不可读、不可改；服务端负责校验完整性并提取声明。

### §1.2 声明

token 至少携带：

| 声明 | 含义 |
|---|---|
| `sub` | 调用方主体标识 |
| `tenant` | 调用方所属租户，唯一 |
| `scopes` | 授权 scope 列表 |
| `exp` | 过期时间 |

### §1.3 校验结果

校验必须拒绝：签名/结构非法、已过期、缺少 `tenant` 或 `scopes` 的 token。被拒绝时返回 401，不进入后续处理。

校验成功后的身份对象是本次请求唯一的租户与权限来源。请求中的任何其它字段都不能替代或扩大它。

## §2 租户绑定（硬约束）

1. 请求路径中的租户 `/scim/v2/{tenant}/...` 是路由的一部分，**必须**与 token 声明的 `tenant` 严格相等（区分大小写）。
2. 不一致时返回 403，且在返回之前不得读写任何存储。
3. 校验通过后，所有仓储调用都必须使用 token 的 `tenant` 作为隔离键；不得使用请求体、query、header 中出现的租户值。
4. 跨租户访问的另一个租户的资源 id 一律按不存在处理（404），不泄露存在性。

路径租户只用于确认「调用方在跟谁说话」，不构成授权依据本身。

## §3 Scope 模型

### §3.1 命名

```
<resource>:<attribute>:<action>
```

`resource` ∈ `users`、`groups`；`attribute` ∈ `profile`、`groups`、`roles`、`active`；`action` ∈ `read`、`write`。

另有粗粒度 scope：`users:*:read` 表示 `users` 下所有属性可读；`users:*:write` 表示所有属性可写。粗粒度 scope 蕴含对应的细粒度 scope。

### §3.2 受控属性

| 属性 | 所需 scope（读） | 所需 scope（写） |
|---|---|---|
| `groups` | `attr:groups:read` | `attr:groups:write` |
| `roles` | `attr:roles:read` | `attr:roles:write` |
| `active` | `attr:active:read` | `attr:active:write` |
| 其它（`userName`、`name.*`、`emails`、`externalId`、Group 的 `displayName`） | `attr:profile:read` | `attr:profile:write` |

Group 的 `members` 归入 `attr:groups:*`。

实际 token 中 scope 以 `attr:<name>:<action>` 的缩写形式出现（例如 `attr:groups:read`），按上表映射。

### §3.3 判定顺序

1. 认证（401）；
2. 租户绑定（403）；
3. 资源定位：不存在或不属于本租户 → 404；
4. 写操作的属性级授权 → 403；
5. 执行。

授权判定必须在产生任何副作用之前完成。

## §4 读取裁剪

响应中的每个属性都必须经过裁剪：调用方没有该属性读权限时，属性不得出现在响应里（包括 `null` 占位）。裁剪在序列化层统一执行，且**先于** `attributes` / `excludedAttributes` 投影。

- 不带 `attr:groups:read` 的 token 读出的 User 资源中不得含 `groups` 键；
- 不带 `attr:roles:read` 时不得含 `roles`；
- 不带 `attr:active:read` 时不得含 `active`；
- Group 的 `members` 同理随 `attr:groups:read`。

投影参数只能进一步收窄，不能绕开上述规则。`schemas`、`id`、`meta` 不受裁剪影响。

## §5 写入授权

### §5.1 写操作种类

`POST`（创建）、`PUT`（整体替换）、`PATCH`（部分更新）都算写入。每个被写入的属性都必须由 token 具备其写 scope。

### §5.2 PATCH 的 path 判定

PATCH 的 `path` 既可能直接点名属性，也可能带值过滤：

- `groups`、`roles`、`active` 直接点名时，按 §3.2 判定；
- `groups[value eq "admins"]`、`members[value eq "<id>"]` 这类带过滤的路径，其**根属性**（方括号之前的部分）决定所需 scope；过滤条件本身不能改变判定结果。例如 `groups[...]` 始终需要 `attr:groups:write`，无论过滤器里写的是什么。
- 无 `path` 的整体 `add`/`replace`，按 `value` 对象的每个顶层键逐个判定。

授权判定必须在应用操作之前完成；不得先写后判，也不得依赖写后回滚来「补救」。

### §5.3 被拒时的行为

scope 不足返回 403，资源不发生任何修改（含同批 Operations 中的其它操作）。错误响应不包含被拒属性的现有值。

### §5.4 版本与条件请求

写入成功后 `meta.version` 加一。携带 `If-Match` 时，只有在版本匹配后才允许写入（scim-profile.md §6）；不匹配返回 412，资源与版本均不改变。

## §6 日志与可观察性

审计日志记录：`sub`、`tenant`、method、path、命中的 scope、结果状态。日志中的 `tenant` 来自校验后的身份对象，不来自请求。

日志不得记录 token 原文或高敏属性的完整值。

## §7 错误映射

| 场景 | 状态 | scimType |
|---|---|---|
| 无 token / 结构非法 / 过期 | 401 | `invalidToken` |
| 路径租户与 token 不一致 | 403 | `tenantMismatch` |
| 缺读 scope 的属性不会出现（非错误） | — | — |
| 缺写 scope | 403 | `insufficientScope` |
| PATCH path 无法定位 / 未知属性 | 400 | `invalidPath` |
| filter 语法错误 | 400 | `invalidFilter` |
| If-Match 不匹配 | 412 | `preconditionFailed` |
