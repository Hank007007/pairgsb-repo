# Lease 规范

lease 表示一次 job 对一份 env 文件的独占使用权。agent 在 job 开始时申请，job 结束或超时后由 sidecar 回收。

## §1 申请

### §1.1 请求

agent 通过 `POST /v1/lease` 提交：

```json
{
  "job": "web-main",
  "pipeline": "web",
  "branch": "main",
  "scopes": ["app", "shared"],
  "ttl_ms": 600000
}
```

- `job`：本次构建的唯一标识，用于幂等与回收。
- `pipeline` / `branch`：策略匹配所需的构建上下文。
- `scopes`：本次希望注入的 scope 列表，必须是流水线声明中允许的 scope 的子集。
- `ttl_ms`：期望的 lease 存活时间，单位毫秒。

### §1.2 有效期

lease 的 ttl 由 job 自身的 `max_duration` 封顶。请求值小于 `max_duration` 时采用请求值；请求值大于 `max_duration` 或缺失时采用 `max_duration`。sidecar 不得发出超过 job `max_duration` 的 lease。

### §1.3 响应

成功时返回：

```json
{
  "lease_id": "…",
  "env_path": "/…/leases/web-main.env",
  "expires_at": "2026-01-01T00:10:00.000Z"
}
```

失败时不生成 env 文件，也不产生 lease。

## §2 生命周期

### §2.1 与 max_duration 的关系

见 §1.2。`max_duration` 由流水线声明提供，不信任 agent 上报的值。

### §2.2 幂等

同一个 job 在 lease 仍然有效期间重复申请，必须返回同一份 lease、同一个 lease id、同一个 env 路径，并且磁盘上只有这一份文件。重复申请不得重新生成文件，也不得延长或缩短已经确定的到期时间。

job 的 lease 已经被回收后再次申，视为一次新的申请。

### §2.3 读取

agent 通过 lease 句柄读取 env 内容。读取只对 lease 的持有者开放。lease 被撤销或到期后，已有句柄不能继续读出内容。

### §2.4 到期回收

lease 到期时，sidecar 必须同时做到：

1. 从内存中移除该 lease；
2. 关闭该 lease 的读取句柄；
3. 从磁盘删除对应的 env 文件。

只删除内存记录是不够的：env 文件不得在 lease 失效后继续留在磁盘上。

## §3 撤销

### §3.1 撤销后的读取

调用 `revoke(lease_id)` 后，该 lease 立即失效。此后任何一次读取都必须立即失败，包括在撤销之前已经取得的读取句柄。撤销同样会删除 env 文件。

### §3.2 未知 lease

对不存在或已回收的 lease 执行撤销是幂等的，不产生副作用。
