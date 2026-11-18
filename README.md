# hopfence

内网南北向转发代理。按路径前缀把请求路由到配置中的固定上游，转发前做一轮头部处理与路由 ACL 判断，转发后写一条访问日志。

- 规范：`docs/forwarding.md`
- 配置项：`docs/routes.md`
- 设计说明：`docs/design.md`

## 目录

```
cmd/hopfence/        进程入口
internal/config/     配置加载与校验
internal/accesslog/  访问日志
internal/proxy/      选路、ACL、头部处理、转发
docs/                文档
config.example.json  示例配置
```

## 构建与运行

```sh
go build -o hopfence ./cmd/hopfence
./hopfence -config config.example.json
```

`go run ./cmd/hopfence -config config.example.json` 亦可。

## 测试

```sh
go test ./...
```

## 配置速览

上游和路由只来自配置文件，运行期不重载。字段说明见 `docs/routes.md`。
