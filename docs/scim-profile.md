# SCIM 2.0 Profile — scimfence

本文描述 scimfence 对上游 HR 系统暴露的 SCIM 2.0 门面行为。文中「必须 / 不得」是规范性要求；实现细节不在本文范围。

## §1 总览

scimfence 位于上游 HR 系统与内部身份库之间。上游以 Bearer token 调用，门面负责：

- 解析 token，确定调用方租户（tenant）与可用 scope；
- 把 SCIM 资源映射到内部身份库记录；
- 把 SCIM filter 编译为内部查询；
- 对高敏属性（`groups`、`roles`、`active`）做独立的读写授权；
- 将内部记录序列化为 SCIM 资源，并按 scope 裁剪。

租户隔离是硬约束：任何一次调用能看到、创建、修改、删除的记录，都必须且只能属于调用方租户。

## §2 资源表示

### §2.1 User

User 资源的顶层属性：

| 属性 | 类型 | 说明 |
|---|---|---|
| `id` | string | 租户内稳定标识，由服务端生成 |
| `userName` | string | 登录名，租户内大小写不敏感唯一 |
| `externalId` | string | 上游系统标识，租户内唯一 |
| `name` | object | `givenName`、`familyName` |
| `emails` | array | 元素为 `{value, type, primary}` |
| `active` | boolean | 高敏属性 |
| `groups` | array | 元素为 `{value, display}`，高敏属性 |
| `roles` | array | 元素为 `{value, display}`，高敏属性 |
| `meta` | object | 见 §2.3 |

### §2.2 Group

Group 资源的顶层属性：`id`、`displayName`、`members`（元素为 `{value, display, type}`）、`meta`。`members` 随 `attr:groups:read` 可见。

### §2.3 meta 与版本

每个资源带 `meta`：

- `resourceType`：`"User"` 或 `"Group"`；
- `created`、`lastModified`：RFC 3339 时间；
- `location`：资源在本服务中的绝对路径；
- `version`：单调递增的版本号，以字符串形式表示，新建资源为 `"1"`，每次成功写入加一。

`meta.version` 是 `If-Match` 的唯一比较依据，见 §6。

## §3 Filter

### §3.1 语法

`GET /Users`、`GET /Groups` 支持 `filter` 查询参数。语法：

```
expr        = or-expr
or-expr     = and-expr ( "or" and-expr )*
and-expr    = not-expr ( "and" not-expr )*
not-expr    = "not" not-expr | primary
primary     = "(" expr ")" | attr-path op value | attr-path "pr"
attr-path   = ATTR ( "[" expr "]" )?
op          = "eq" | "ne" | "co" | "sw" | "ew" | "gt" | "ge" | "lt" | "le"
```

ATTR 为属性名，可含 `.` 表示子属性（如 `name.givenName`）。属性名与操作符均大小写不敏感；属性名按 SCIM 规定大小写不敏感解析。

`pr` 表示「存在且非空」。

### §3.2 字符串字面量

字符串值以双引号包围，遵循 JSON 字符串转义：`\"`、`\\`、`\/`、`\b`、`\f`、`\n`、`\r`、`\t`、`\uXXXX`。

引号内的内容一律是字面值：转义后的引号不结束字符串，也不得被解释为语法。例如 `userName eq "a\"b"` 比较的是三个字符的名字 `a"b`。解析器必须读完整个字符串字面量，任何多余的、无法构成完整表达式的尾随 token 都应导致 400。

### §3.3 比较语义

- `eq` / `ne`：字符串比较大小写不敏感；布尔与数字按值比较。
- `co` / `sw` / `ew`：对字符串做子串判断，大小写不敏感。
- `gt` / `ge` / `lt` / `le`：字符串按码点序、数字按数值比较。
- `pr`：属性存在、非 `null`、非空字符串、非空数组。

`co` / `sw` / `ew` 的右值是纯字面量。内部查询层使用 LIKE 风格的式串，其中 `%` 与 `_` 是元字符；编译器必须对字面值中的元字符做转义，使它们只匹配自身。用户输入的 `%`、`_` 不得获得通配含义。

### §3.4 租户约束（硬约束）

编译器必须把调用方租户条件与客户端表达式做逻辑与，且该条件必须支配整个表达式：

- 顶层是 `and`、`or`、`not`、括号嵌套或任意组合时，结果查询都必须是 `tenant = <调用方租户> AND <客户端表达式>`；
- `or`/`not` 的任何分支都不得绕过租户条件；
- 客户端无法通过 filter 读取、推断其他租户的记录；跨租户构造返回空集。

客户端表达式在编译前视为不可信输入，不能直接作为内部查询的顶层条件使用。

### §3.5 错误

语法错误、未知操作符、未闭合的字符串或括号、尾随垃圾字符一律返回 400，响应体为 SCIM Error（§7）。不得把解析失败的 filter 当作「无条件」处理。

## §4 PATCH

### §4.1 请求体

`PATCH /Users/{id}`、`PATCH /Groups/{id}` 接受：

```json
{
  "schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
  "Operations": [
    {"op": "add", "path": "roles", "value": [{"value": "admin"}]},
    {"op": "replace", "path": "name.givenName", "value": "Ada"},
    {"op": "remove", "path": "emails[type eq \"work\"]"}
  ]
}
```

`op` 取 `add`、`replace`、`remove`，大小写不敏感。`Operations` 中任一操作失败时，整个 PATCH 不得产生任何持久化修改（原子性）。

### §4.2 path 形式

`path` 支持三种形式：

1. 属性路径：`userName`、`name.givenName`、`emails`；
2. 带值过滤的属性路径：`emails[type eq "work"]`、`groups[value eq "admins"]`；
3. 多值成员定位：`members[value eq "<id>"]`（Group）。

值过滤中的字符串同样遵循 §3.2 的转义规则，操作符遵循 §3.3。

### §4.3 定位与写入

PATCH 分两个阶段：

1. **定位（只读）**：按 path 在资源上解析出目标位置。属性名必须是资源 schema 中已定义的属性；子属性必须存在于父属性之下；带值过滤的路径必须能唯一命中至少一个现有元素。定位失败，或属性名拼写不在 schema 内，返回 400（`invalidPath`），不得创建新属性或新元素来「凑」出目标。
2. **写入**：在已定位的目标上执行操作。

`replace` 要求目标已存在；目标不存在时返回 400，而不是新增。`remove` 要求目标存在，不存在时返回 400。`add` 可以向已知的多值属性追加元素，或创建 schema 中已定义、当前缺失的可选属性；对未知属性名同样返回 400。

无 `path` 的 `add`/`replace` 表示对 `value` 对象的每个顶层属性执行 `replace`，其语义与带 path 时一致（目标必须已存在）。

### §4.4 remove 与空组

对 Group 执行 `remove members[value eq "<id>"]` 后，若 `members` 变为空数组，该 Group 视为已无保留意义，必须在同一操作中删除。响应为 204，后续按 id 读取返回 404。

### §4.5 并发

PATCH 与 PUT 的版本推进规则见 §2.3 与 §6。

## §5 查询参数与分页

### §5.1 attributes / excludedAttributes

`attributes` 与 `excludedAttributes` 为逗号分隔的属性名列表，大小写不敏感；二者互斥，同时出现返回 400。

投影在 scope 裁剪（见 attribute-authz.md §4）之后执行：

- `attributes`：只返回列出且调用方有权读取的属性，外加 `schemas`、`id`、`meta`；
- `excludedAttributes`：从有权读取的属性中排除列出的属性，`schemas`、`id`、`meta` 不可排除；
- 投影不能使调用方看到本无权读取的属性。例如没有 `attr:groups:read` 的 token 使用 `attributes=userName` 时，结果中不得出现 `groups`。

### §5.2 分页

`startIndex` 从 1 开始，缺省为 1；小于 1 或非整数返回 400。

`count` 为期望的本页条数，缺省 100；必须为整数；上限 100。超过 100 时按 100 返回本页。

响应：

```json
{
  "schemas": ["urn:ietf:params:scim:api:messages:2.0:ListResponse"],
  "totalResults": 42,
  "startIndex": 1,
  "itemsPerPage": 42,
  "Resources": []
}
```

- `totalResults` 是满足 filter 且属于调用方租户的记录总数，与 `count` 无关；不得用请求的 `count` 冒充；
- `itemsPerPage` 是本次实际返回的条数，`itemsPerPage = Resources.length`；
- 各页按 `startIndex` 连续切分，不重不漏；`startIndex` 超过总数时返回空 `Resources`，`totalResults` 仍为总数。

## §6 条件请求

写操作（PATCH、PUT、DELETE）可以携带 `If-Match`。服务端将 `If-Match` 的值按 RFC 7232 解析为实体标签，与当前资源的 `meta.version` 比较（弱比较符 `W/` 允许，引号不计入比较）。

- 匹配：执行写入，版本加一；
- 不匹配：返回 412 `Precondition Failed`，资源保持调用前的状态，版本不变；
- 未携带 `If-Match`：按 §4 正常执行。

响应中通过 `ETag: "<meta.version>"` 返回当前版本。

## §7 错误

错误响应：

```json
{
  "schemas": ["urn:ietf:params:scim:api:messages:2.0:Error"],
  "status": "400",
  "scimType": "invalidFilter",
  "detail": "human readable"
}
```

常用状态码：

| 状态 | 场景 |
|---|---|
| 400 | filter/path/请求体语法错误、非法分页参数、未知属性 |
| 401 | 缺少或无法识别 token |
| 403 | token 有效但 scope 不足（含属性级写权限） |
| 404 | 资源不存在，或不属于调用方租户 |
| 412 | If-Match 不匹配 |
| 409 | `userName` / `externalId` 冲突 |

## §8 端点

所有端点前缀 `/scim/v2/{tenant}`：

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/Users` | 列表，支持 filter、分页、投影 |
| POST | `/Users` | 创建 |
| GET | `/Users/{id}` | 读取 |
| PATCH | `/Users/{id}` | 部分更新 |
| DELETE | `/Users/{id}` | 删除 |
| GET/POST | `/Groups`、`/Groups/{id}`、`/Groups/{id}/PATCH`、`/Groups/{id}/DELETE` | 同上 |

路径中的 `{tenant}` 必须与 token 所属租户一致；不一致时返回 403，且不得访问任何资源（见 attribute-authz.md §2）。
