# scopejoin

计算一个请求同时携带多张凭证时的有效 scope 与角色。

规范见 [`docs/grant-merge.md`](docs/grant-merge.md)。

## 布局

```
cmd/scopejoin/       命令行入口：读取 JSON 请求，输出有效授权
internal/grant/      合并逻辑
  parse.go           凭证解析
  scope.go           scope 集合运算与特殊 scope 过滤
  role.go            角色合并
  deleg.go           委托链收窄
  effective.go       最终结果
  clock.go           过期与时钟偏移判定
docs/grant-merge.md  规范
```

## 使用

```sh
go test ./...
go run ./cmd/scopejoin request.json
```

请求格式：

```json
{
  "now": "2026-01-01T12:00:00Z",
  "credentials": [
    {
      "kind": "oidc_access_token",
      "payload": {
        "subject": "user-1",
        "scope": "openid read",
        "roles": ["reader", {"name": "admin", "deny": true}],
        "expires_at": "2026-01-01T13:00:00Z"
      }
    }
  ]
}
```

`kind` 取 `oidc_access_token`、`service_account`、`delegated`；匿名凭证以
`{"anonymous": true, "subject": "guest"}` 表示，不带 payload。

## 约定

- 只使用标准库。
- 输出中的 scope 与角色是稳定排序的。
