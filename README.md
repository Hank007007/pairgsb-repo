# permcache

permcache 是策略决策点（PDP）中的决策缓存层。它从策略 bundle 装载规则，对授权请求做
subject / action / resource / tenant 匹配，并把求值结果按策略 revision 缓存起来。

规范行为见 [docs/pdp-cache.md](docs/pdp-cache.md)。

## 目录

```
cmd/permcache       可运行的示例程序
internal/policy     bundle 装载、revision 管理与规则匹配
internal/cache      缓存键构造与决策存储
internal/decide     求值引擎与撤销处理
docs                设计说明
```

## 构建与测试

```
go build ./...
go test ./...
```

## 运行示例

```
go run ./cmd/permcache
```

示例程序会装载一组内置策略，依次对若干请求给出决策并以 JSON 输出，包含是否命中缓存。
