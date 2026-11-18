# hopfence 配置说明

配置文件为 JSON。所有字段均为启动时读取，运行期不重载。未知字段会导致启动失败。

## 顶层字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `listen` | string | 否 | `127.0.0.1:8080` | 监听地址，`host:port` 形式 |
| `access_log` | string | 否 | 空 | 访问日志文件路径；为空时写标准输出 |
| `timeout_seconds` | int | 否 | `30` | 单次上游请求的总超时 |
| `routes` | array | 是 | — | 路由列表，至少一条 |

## 路由字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `name` | string | 是 | — | 路由名，全局唯一，用于日志与运维识别 |
| `prefix` | string | 是 | — | 路径前缀，必须以 `/` 开头，不得含 `?` 或 `#` |
| `upstream` | string | 是 | — | 上游基地址，`http` 或 `https`，不得含查询串或片段 |
| `allow` | array | 否 | 空 | 允许的客户端网段；为空表示不限制 |
| `deny` | array | 否 | 空 | 拒绝的客户端网段；优先于 `allow` |
| `deny_paths` | array | 否 | 空 | 该路由拒绝服务的路径前缀，必须以 `/` 开头 |

`allow` / `deny` 的元素可以是 CIDR（`10.0.0.0/8`、`2001:db8::/32`），也可以是单个 IP（`127.0.0.1`、`::1`）。单个 IP 视为只包含该地址的前缀。

## 示例

```json
{
  "listen": "127.0.0.1:8080",
  "access_log": "",
  "timeout_seconds": 15,
  "routes": [
    {
      "name": "static",
      "prefix": "/static",
      "upstream": "http://127.0.0.1:9001",
      "deny_paths": ["/static/private"]
    },
    {
      "name": "admin",
      "prefix": "/admin",
      "upstream": "http://127.0.0.1:9002",
      "allow": ["10.0.0.0/8", "127.0.0.1"],
      "deny": ["10.9.0.0/16"],
      "deny_paths": ["/admin/ops"]
    },
    {
      "name": "api",
      "prefix": "/",
      "upstream": "http://127.0.0.1:9003"
    }
  ]
}
```

## 校验规则

- `listen` 必须能被解析为 `host:port`。
- `timeout_seconds` 必须为正数。
- 路由 `name` 不得重复。
- `prefix`、`deny_paths` 的每一项都必须以 `/` 开头且不含 `?`、`#`。
- `upstream` 的 scheme 必须是 `http` 或 `https`，且 host 非空。
- `allow`、`deny` 的每一项都必须能被解析为 CIDR 或 IP。

任一条不满足时进程拒绝启动，并在标准错误输出中说明出错位置。
