# outfetch

内部用的出方向抓取服务。调用方把 URL 丢过来，我们替它发一次 GET，
只回状态码、字节数和正文的 SHA-256 —— 正文不落盘、不回传。

## 目录

| 路径 | 职责 |
|---|---|
| `cmd/outfetch/` | 服务入口，读命令行参数 |
| `internal/api/` | HTTP 接口：`POST /fetch`、`GET /healthz` |
| `internal/policy/` | 准入策略：主机允许/拒绝名单、端口白名单、私网拦截 |
| `internal/fetcher/` | 真正发请求的客户端，读响应体、算摘要 |
| `docs/egress-policy.md` | **出方向访问安全规范，以它为准** |
| `docs/incident-reports.md` | 这两周报上来的问题摘录 |

## 跑测试

```bash
go test ./...
```

只用标准库（Go 1.22+），没有第三方依赖。

## 起服务

```bash
go run ./cmd/outfetch -addr :8088 -allow shop.example.com,*.example.org -ports 80,443
```

```bash
curl -s -X POST localhost:8088/fetch \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://shop.example.com/ping"}'
```

`-allow` 留空时，服务按规范第 2 条只按私网拦截，不做主机名白名单。
