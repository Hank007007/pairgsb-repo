# sharefence ACL 模型

本文定义 sharefence 授权层必须满足的行为契约。文中只描述「什么算正确」，不规定实现方式。

## 1. 范围

sharefence 负责两件事：

1. 在文件树节点上解析 ACL，回答「主体 P 在节点 N 上实际拥有什么角色」。
2. 校验发往外部的一次性分享链接，回答「这个 token 现在能否用于访问某个路径」。

授权判定只依赖存储中的节点、ACL 条目、组成员关系和分享链接，不访问文件系统。

## 2. 角色

### 2.1 角色集合

合法角色共四个，按权限从低到高排列：

| 角色 | rank | 说明 |
|---|---|---|
| `viewer` | 0 | 只读 |
| `commenter` | 1 | 只读并可评论 |
| `editor` | 2 | 可编辑内容 |
| `owner` | 3 | 可管理权限与分享 |

除这四个字符串以外的值都不是合法角色。

### 2.2 角色偏序与 `atLeast`

`atLeast(granted, required)` 回答「`granted` 是否覆盖 `required`」。当且仅当两者都合法角色，且 `rank(granted) >= rank(required)` 时结果为真；任一参数非法时结果为假。

因此：

- `atLeast('editor', 'viewer')` 为真；
- `atLeast('viewer', 'editor')` 为假；
- `atLeast('owner', 'owner')` 为真。

角色高低只由上表的 rank 决定，与角色名的字符串顺序无关。

### 2.3 最高角色

`maxRole(list)` 返回列表中 rank 最高的合法角色；列表为空或不含合法角色时返回 `null`。非法值不参与比较。

## 3. 路径与子树

### 3.1 规范形式

所有对外接受的路径都先转换为规范形式再参与比较。规范形式满足：

- 以 `/` 开头；
- 除根 `/` 外不以 `/` 结尾；
- 不含空段、`.` 段或 `..` 段；
- 根路径为 `/`。

### 3.2 规范化规则

`normalize(path)` 按以下顺序处理输入：

1. 非字符串或空字符串视为 `/`；
2. 不以 `/` 开头的相对路径按从根开始处理，即在前面补 `/`；
3. 连续多个 `/` 合并为一个；
4. `.` 段直接丢弃；
5. `..` 段回退到上一层；已经在根时继续回退仍停留在根；
6. 去掉末尾多余的 `/`（根除外）。

规范化只做纯字符串运算，不查询节点是否存在，也不解析符号链接。`..` 在任何子树判断之前即被消解，例如 `/team/../../finance` 的规范形式是 `/finance`。

### 3.3 子树判断

`isInside(ancestor, candidate)` 判断 `candidate` 是否位于 `ancestor` 的子树内（包含 `ancestor` 自身）。判定基于规范化后的路径分段，而不是原始字符串前缀：

- 两侧都先按 §3.2 规范化；
- 相等时为真；
- 否则 `candidate` 必须以 `ancestor` 的完整分段序列开头，并且后面紧跟 `/`；
- 根 `/` 包含所有路径。

因此 `/team` 包含 `/team` 与 `/team/design`，不包含 `/team-evil`、`/teamwork` 或 `/other/team`。

## 4. ACL 解析

### 4.1 条目

一条 ACL 条目的结构为：

```
{ nodeId, principal, role, deny }
```

- `nodeId`：条目挂载的节点；
- `principal`：`user:<id>` 或 `group:<id>`；
- `role`：§2.1 中的合法角色；
- `deny`：布尔值，缺省为 `false`。

### 4.2 最近显式条目

`effectiveRole(store, userId, nodeId)` 返回主体在该节点上的实际角色，或 `null` 表示无权限。

解析从目标节点开始，沿父链向根逐层进行，目标节点自身距离为 0，父节点距离为 1，依此类推。找到**第一个至少存在一条适用于该主体的条目的层级**后，该层的结果即为最终结果，不再继续更远的祖先查找。若整条父链都没有适用条目，结果为 `null`。

适用于主体的条目包括：`principal` 等于 `user:<userId>` 的条目，以及 `principal` 为 `group:<g>` 且该用户属于组 `g` 的条目。

### 4.3 deny 与同层优先级

在选定的层级上：

- 只要存在任意一条适用的 `deny: true` 条目，结果就是 `null`，即使同层还有 allow 条目；
- 否则结果取该层所有适用 allow 条目的最高角色（§2.3）。

`deny` 条目的 `role` 字段不参与判定。

因为解析在最近的层级停止，deny 会沿父链向下传递：若 `/proj` 允许某用户 `editor`，而 `/proj/secret` 显式 deny 该用户，则该用户在 `/proj/secret` 及其所有后代上都没有权限，除非在更近的节点上又有一条显式 allow。

### 4.4 组

组是扁平的成员集合，不嵌套。解析某一层时，用户通过组获得的条目与直接授予用户的条目地位相同：同层同时存在用户条目和组条目时，一起参与 §4.3 的判定，allow 取其中最高角色，任意一条 deny 即可否决。

### 4.5 权限检查

`hasRole(store, userId, nodeId, required)` 在 `effectiveRole` 为 `null` 时返回 `false`，否则返回 `atLeast(effectiveRole, required)`。角色检查必须遵循 §2.2 的偏序。

## 5. 分享链接

### 5.1 链接字段

一条分享链接至少包含：

| 字段 | 含义 |
|---|---|
| `token` | 不透明随机串，外部凭此访问 |
| `nodeId` | 链接挂载的节点，链接的作用范围是该节点的子树 |
| `role` | 持链接者获得的角色 |
| `issuedBy` | 签发人用户 id |
| `expiresAt` | ISO 8601 UTC 字符串或 `null` |
| `revoked` | 是否已撤销 |
| `createdAt` | 签发时间的 ISO 8601 UTC 字符串 |
| `passwordSalt` / `passwordHash` | 密码校验材料；无密码时均为 `null` |

### 5.2 签发

签发时必须满足：

- `nodeId` 指向已存在的节点；
- `role` 是合法角色；
- 签发人在该节点上当前至少持有链接角色（§2.2、§4）。不满足时签发失败，不产生链接；
- `expiresAt` 为 `null` 或可解析为合法时刻；非法值签发失败。

签发产生的链接初始为未撤销状态。

### 5.3 token 与密码

token 必须使用密码学安全的随机源生成，且不可从链接其他字段推导。

密码不得以明文存储。设置密码的链接在每次校验时都要求请求方提供匹配的密码；未提供与不匹配都必须拒绝。未设置密码的链接忽略请求中的密码字段。

### 5.4 过期

`expiresAt` 表示绝对时刻（ISO 8601 UTC 字符串）。校验发生在当前时刻 `now` 时，若 `expiresAt` 非空且 `now >= expiresAt`，链接视为过期并拒绝。`expiresAt` 为 `null` 表示永不过期。判定必须以两个绝对时刻先后关系为准。

### 5.5 签发人当前权限

每次校验都重新计算签发人在链接节点上的实际角色，并要求该角色存在且至少覆盖链接的 `role`（§2.2）。签发人被移出 ACL、被显式 deny、或通过组获得的权限被收回后，链接立即失效，无需等到撤销或过期。链接的有效性对应签发人此刻的权限。

### 5.6 作用域

链接只允许访问其节点子树内的路径。请求携带目标路径时，先按 §3.2 规范化，再用 §3.3 判断目标是否位于链接节点子树内，判定以规范化后的路径分段为边界。

例如挂在 `/team` 上的链接不能访问 `/team-evil/roadmap.md`，也不能通过 `/team/../../finance/q3.xlsx` 访问 `/finance/q3.xlsx`。

### 5.7 校验结果

校验返回 `{ ok, reason, link }`：

- 成功时 `ok` 为 `true`，并返回链接对象；
- 失败时 `ok` 为 `false`，`reason` 为下列之一，按此优先级判定：

| reason | 条件 |
|---|---|
| `unknown_token` | token 不存在 |
| `revoked` | 链接已撤销 |
| `expired` | 已过期（§5.4） |
| `password_required` | 链接有密码但请求未提供 |
| `bad_password` | 提供的密码不匹配 |
| `issuer_revoked` | 签发人当前角色不满足 §5.5 |
| `out_of_scope` | 目标路径不在链接子树内（§5.6） |

所有条件必须同时满足；任一失败即拒绝。

## 6. 目录列举

`visibleChildren(store, userId, folderId)` 返回调用方在目录中可见的子节点列表：

1. 调用方必须在目录本身上拥有至少 `viewer` 的角色（即 `effectiveRole` 非 `null`）；否则返回空数组。
2. 每个子节点必须独立按 §4 解析权限；只有调用方在该子节点上的 `effectiveRole` 非 `null` 时才出现在结果中。
3. 子节点的可见性以该子节点的解析结果为准。子节点自身或其更近祖先上的 deny 会让该子节点从结果中消失，即使调用方能够读取目录本身。
4. 结果按节点 `name` 升序排列，便于稳定比较。

## 7. 存储

内存存储包含：

- `nodes`：`Map<nodeId, node>`，`node` 为 `{ id, name, kind, parentId }`，根节点固定存在；
- `acl`：条目数组，按加入顺序保存；
- `groups`：`Map<groupId, Set<userId>>`；
- `shares`：`Map<token, link>`。

节点和 ACL 条目的字段形状是稳定契约；存储层不做持久化。同一节点上可以存在多条条目，包括同一主体的多条 allow 或 allow 与 deny 并存。

## 8. 边界与非目标

- 不提供 HTTP 层、会话管理或文件内容读取；
- 不做持久化、缓存或并发控制；
- 组不支持嵌套；
- 不解析符号链接，路径规范化是纯字符串运算；
- 不定义 owner 的隐式特权：所有权限都必须来自 ACL 条目。

## 9. 接口摘要

以下签名为对外契约，函数名与返回形状不再变动。

```
effectiveRole(store, userId, nodeId)              -> role | null
hasRole(store, userId, nodeId, required)          -> boolean
atLeast(granted, required)                        -> boolean
maxRole(roles)                                    -> role | null

visibleChildren(store, userId, folderId)          -> node[]

normalize(path)                                   -> string
isInside(ancestor, candidate)                     -> boolean

issueLink(store, { nodeId, issuedBy, role, password, expiresAt }) -> link
verifyLink(store, token, { password, targetPath })                -> { ok, reason, link }
revokeLink(store, token)                                          -> boolean
```

`verifyLink` 的 `password` 与 `targetPath` 均为可选参数；`targetPath` 省略时不执行作用域判断。
