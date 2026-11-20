# verbgate

自建控制面的授权 webhook。API server 把每个请求打包成 SubjectAccessReview 发到 `/authorize`，verbgate 返回 allow / deny / no-opinion。

- 语义规范：[`docs/authz-semantics.md`](docs/authz-semantics.md)
- 设计说明：[`docs/design.md`](docs/design.md)
- 历史事件：[`docs/incident-reports.md`](docs/incident-reports.md)

## 目录结构

```
cmd/verbgate/          可执行入口
internal/authz/        规则匹配、决策汇总、请求/响应结构
internal/subject/      groups.yaml 解析与组展开
internal/store/        角色与绑定快照
internal/cache/        判定缓存
internal/webhook/      HTTP 入口
examples/snapshot.json 示例快照
groups.yaml            示例组成员关系
```

## 运行

```sh
go run ./cmd/verbgate -addr :8080 -groups groups.yaml -snapshot examples/snapshot.json
```

发起一次判定：

```sh
curl -s localhost:8080/authorize -d '{"user":"alice","verb":"get","apiGroup":"","resource":"pods","namespace":"platform","name":"web-1"}'
```

## 测试

```sh
go test ./...
```

仅使用 Go 标准库。
