# SSH CA 签发规范（certbrace）

本文档由运维平台与基础设施组共同维护，是 certbrace 签发行为的规范来源。
凡代码行为与本文档不一致之处，以本文档为准。

约定：

- 时间一律使用 Unix 时间戳（自 UTC 1970-01-01T00:00:00Z 起的秒数），除非某处明确写作毫秒。
- 请求与响应使用 JSON。
- “拒绝”指不签发证书，返回 HTTP 400（服务内部故障返回 5xx），响应体中
  `error` 为稳定的错误码，`message` 为人类可读说明。
- `MUST` / `MUST NOT` 为强制性要求。

---

## 1. 请求与响应

### 1.1 签发请求

| 字段 | 必填 | 说明 |
|---|---|---|
| `role` | 是 | 申请单上的角色名，取值见第 2 节 |
| `publicKey` | 是 | 申请者的公钥。接受 `ssh-ed25519 AAAA... comment` 形式的 authorized_keys 行，也接受 SPKI PEM。当前仅支持 Ed25519 |
| `principals` | 否 | 申请证书携带的登录名列表。缺省时使用 role 的完整允许集 |
| `ttlSeconds` | 否 | 申请的有效期（秒）。缺省时使用 role 的默认值 |
| `criticalOptions` | 否 | critical options 对象，字段见第 5 节 |

字段名同时接受 `snake_case` 与 `camelCase` 两种写法：`public_key` / `publicKey`、
`ttl_seconds` / `ttlSeconds`、`critical_options` / `criticalOptions`。

### 1.2 签发结果

成功时返回一张 OpenSSH 用户证书，以及本次签发所用的 CA 信息：

- 证书文本（authorized_keys 形式，首列 `ssh-ed25519-cert-v01@openssh.com`）
- CA key id 与 CA 公钥
- 自检结果，见第 7 节

---

## 2. 角色与 principal

角色决定该证书可以以哪些 principal 登录。这是整个签发链路上唯一的授权
来源，sshd 侧只做证书签名校验，不再做 principal 与角色之间的关联。

| role | 允许的 principal | 默认有效期 |
|---|---|---|
| `breakglass` | `root` | 900 秒 |
| `ci` | `ci`, `deploy` | 1800 秒 |
| `deploy` | `deploy` | 3600 秒 |
| `ops` | `ops` | 7200 秒 |
| `sre` | `sre`, `ops` | 14400 秒 |

规则：

1. `MUST` 校验请求中的每个 principal 都属于该 role 的允许集。
2. 只要有一个 principal 不在允许集内，`MUST` 拒绝签发该请求，并在响应中
   指出越权的那个 principal（`details.principal`）。不得签发“去掉越权项
   之后”的证书。
3. 未提供 `principals` 时，`MUST` 使用该 role 的完整允许集。
4. 允许的 principal 集合 `MUST NOT` 被请求内容扩大；请求只能缩小它。
5. 未知 role `MUST` 拒绝。

> 例：role 为 `deploy` 的申请携带 `["deploy", "root"]` 必须失败，错误信息
> 指明 `root`；携带 `["deploy"]` 或不带 `principals` 则成功。

---

## 3. 有效期

1. 未提供 `ttlSeconds` 时，使用第 2 节表中该 role 的默认有效期。
2. 任意证书的有效期 `MUST NOT` 超过 **8 小时（28800 秒）**，与 role 的
   默认值无关。请求超过上限时 `MUST` 拒绝，错误码为 `ttl_exceeded`。
3. `ttlSeconds` 必须为正整数；非正数或非数字 `MUST` 拒绝。
4. 证书中的 `valid_after` 取签发时刻，`valid_before` 为 `valid_after + 有效期`。
5. 允许为 `valid_after` 留出不超过 60 秒的时钟偏差余量，除此之外不得提前生效。

---

## 4. 证书字段与编码

证书遵循 OpenSSH PROTOCOL.certkeys 定义的 `ssh-ed25519-cert-v01@openssh.com`
格式。签名覆盖“证书主体”（从 key type 到签名公钥，含签名公钥），签名本身
追加在末尾。

字段顺序与编码：

| 字段 | 编码 |
|---|---|
| key type | string，`ssh-ed25519-cert-v01@openssh.com` |
| nonce | string，32 字节随机数 |
| public key | string，申请者公钥的 OpenSSH blob |
| serial | uint64，见下 |
| type | uint32，用户证书为 1 |
| key id | string |
| principals | name-list（string 包裹的一串 string） |
| valid after | uint64，**秒** |
| valid before | uint64，**秒** |
| critical options | string 包裹的 option 列表 |
| extensions | string 包裹的 extension 列表，可为空 |
| reserved | string，空 |
| signature key | string，CA 公钥的 OpenSSH blob |
| signature | string，内容为 (string 算法名, string 裸签名) |

### 4.1 serial

1. `serial` 是 **uint64**，按大端序写入，占用完整的 8 字节。
2. 每张证书的 serial `MUST` 唯一，且随签发顺序 **严格单调递增**。
3. serial 不得恒为 0；KRL 依据 serial 吊销，重复的 serial 会导致错误吊销。
4. serial 的分配状态需要跨进程重启保留，避免重启后重新发放已用过的号段。

`ssh-keygen -L` 展示的 `Serial:` 必须能与请求一一对应。

### 4.2 时间字段

`valid_after` / `valid_before` 在证书里是 uint64 秒。任何读取证书的地方都
必须按秒解释这两个字段。

---

## 5. critical options

1. 支持的字段为 `force_command`，其值非空字符串，编码到证书的
   critical options 中，名称写作 `force_command`。
2. 请求中同时接受 `force_command` 与 `forceCommand` 两种写法，二者语义完全
   相同，最终都落到证书的 `force_command` 上。
3. `MUST` 拒绝无法识别的 critical option 名称，包括拼写错误（例如
   `force_comand`、`forcecommand`）以及未定义的字段。错误码为
   `invalid_options`，响应中指出具体字段名。
4. 值为空字符串、非字符串或 null 时 `MUST` 拒绝。
5. 除 `force_command` 外，本服务当前不写入其他 critical option。
6. 编码细节见 OpenSSH PROTOCOL.certkeys：critical options 字段本身是一个
   string，其内容为若干 (string 名称, string 数据) 对；`force_command`
   的数据为命令本身，sshd 读取 `force-command` 时按原样使用。

---

## 6. CA 密钥与轮换

1. CA 私钥位于 `ca/`，文件名为 `<key id>.key`，内容为 PEM。
2. key id 形如 `certbrace-v<N>`；轮换时旧文件的 key id 追加 `.retired`，
   即 `certbrace-v1.retired.key`。
3. 每个 CA 私钥都有对应的 OpenSSH 公钥（`<key id>.pub`），提交给 sshd 的
   `TrustedUserCAKeys`。轮换后 sshd 只信任新的活跃 CA 公钥。
4. **活跃 CA** 定义为：key id 不带 `.retired` 后缀、版本号最大的一把。
   同一目录下可以同时存在活跃 key 与任意多把 retired key。
5. 签发 `MUST` 使用活跃 CA。只要存在可用的活跃 CA，就 `MUST NOT` 使用
   retired key 签发。
6. 目录中没有任何可用活跃 CA 时 `MUST` 失败，不得回退到 retired key。
7. retired key 仅用于验证轮换前签发的历史证书。
8. 目录内的文件枚举顺序不构成任何语义；选 key 的结果不得依赖文件系统
   返回顺序。

---

## 7. 自检

每次签发在返回之前都要用本次使用的 CA 公钥对证书做一遍自检：

1. 重新解析证书，确认编码可以完整往返。
2. 用 CA 公钥验证证书签名。签名不通过时自检失败。
3. 校验有效期：`valid_after` 与 `valid_before` 按秒解释，与当前时间比较。
   刚签发的证书必须落在有效窗口内——不能因为单位混用被判为“已过期”。
4. 校验有效期长度不超过第 3 节的上限。
5. 自检结果随响应返回；自检失败时 `MUST` 在响应中明确标记，运维人员必须
   能看到。

`ssh-keygen -L -f <cert>` 的输出必须与上述字段一致，可用于人工核对。

---

## 8. 与 sshd 的约定

- `TrustedUserCAKeys` 指向活跃 CA 的 `.pub`。
- `AuthorizedPrincipalsFile` 以 principal 名匹配，不使用证书里的 key id。
- 证书的 principal 若不在第 2 节允许集内，即视为授权越界，属于签发侧缺陷。
