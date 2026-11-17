# keyladder — 设计说明

`keyladder` 是部署在 API 网关后面的密钥生命周期服务。一个实例服务多家业务方：
业务方在控制台签发密钥，网关在每个请求上调用 `verify`，拿到归属（`ownerId`）与
权限（`scopes`）之后决定是否放行。

规范条款见 [`key-policy.md`](./key-policy.md)。本文只交代系统结构与各组件的职责，
不规定实现细节。

## 组件

| 组件 | 文件 | 职责 |
|---|---|---|
| 密钥格式 | `src/key-format.js` | 解析与拼装 `kl_<env>_<id8>_<secret>` |
| 存储 | `src/store.js` | 持久化密钥记录，提供按 `id8` 读取与按出示密钥查找 |
| 缓存 | `src/cache.js` | 进程内 LRU，按 `id8` 缓存记录，供网关热路径复用 |
| 签发 | `src/issue.js` | 生成 `id8` 与 `secret`，落库并返回一次性明文 |
| 轮换 | `src/rotate.js` | 为已有密钥签发后继，并给原记录打上宽限期 |
| 收回 | `src/revoke.js` | 将记录标记为已收回 |
| 验证 | `src/verify.js` | 处理网关的验证调用，返回归属、权限或失败原因 |
| 服务装配 | `src/service.js` | 把上述组件按配置组装成一个实例 |

## 生命周期

```
issue ──▶ active ──rotate──▶ active(grace) ──▶ expired
             │                    │
             └──── revoke ────────┴──▶ revoked
```

- `active`：密钥可验证。
- `active(grace)`：轮换后的原密钥；在 `graceUntil` 之前可用，之后不可用。
- `expired`：宽限期已过。
- `revoked`：已收回，终态。

## 记录字段

| 字段 | 含义 |
|---|---|
| `id8` | 查找标识，全局唯一 |
| `env` | 密钥所属环境 |
| `ownerId` | 业务方标识 |
| `scopes` | 权限列表 |
| `createdAt` | 签发时间 |
| `rotatedAt` / `graceUntil` / `successorId8` | 轮换信息 |
| `revoked` / `revokedAt` | 收回信息 |

记录中与密钥本体相关的内容必须遵循 `key-policy.md` §1。

## 配置

通过 `loadConfig()` 读取，可用环境变量或显式传参覆盖：

| 配置 | 环境变量 | 默认值 |
|---|---|---|
| 数据文件 | `KEYLADDER_DATA_FILE` | `data/keys.json` |
| 服务端 pepper | `KEYLADDER_PEPPER` | 开发用常量 |
| 宽限期 | `KEYLADDER_GRACE_MS` | 24 小时 |
| 缓存条目上限 | `KEYLADDER_CACHE_SIZE` | 256 |
| 日志级别 | `KEYLADDER_LOG_LEVEL` | `info` |

生产部署必须通过 `KEYLADDER_PEPPER` 注入 pepper；仓库中的默认值只用于本地开发。

## 验证结果

`verify` 永远返回结构化结果，不抛异常表示可预期的失败。成功时携带 `ownerId`
与 `scopes`，失败时携带 `reason`（取值见 `key-policy.md` §7）。

## 命令行

```
node bin/keyladder.js issue --owner <id> --scopes <a,b> [--env <env>]
node bin/keyladder.js verify <key> --env <env>
node bin/keyladder.js rotate <id8>
node bin/keyladder.js revoke <id8>
node bin/keyladder.js list
node bin/keyladder.js demo
```

`--data <file>` 指定数据文件，`--log-level <level>` 调整日志。`demo` 在临时目录中
走一遍完整生命周期并打印结果，不读写仓库内的数据文件。

## 测试

```
node --test
```

测试使用 `node:test` 与 `node:assert/strict`，每个用例在系统临时目录中创建独立的
数据文件，通过 `createFixedClock()` 注入可控时钟。
