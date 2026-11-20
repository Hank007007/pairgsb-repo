# strictcast

`strictcast` 是内部 API 网关使用的请求参数校验库。每个接口注册一份 Shape
（字段名、类型、是否必填、字节上限、字符集类别、枚举）；网关把 JSON body
或 form 的 `map[string][]string` 交给 `gate.Bind`，得到强类型结果，或得到
按字段路径组织的 400。

行为的唯一依据是 [`docs/shape-contract.md`](docs/shape-contract.md)。

## 目录

```
internal/shape/shape.go      Shape 定义、路径解析与 Expand
internal/shape/coerce.go     CoerceInt / CoerceBool / CoerceString
internal/shape/charset.go    CheckClass 与 NFKC 归一化
internal/shape/size.go       LimitBytes
internal/shape/registry.go   本地 demo 的 Shape 注册表
internal/gate/handler.go     Bind 主流程与错误包装
cmd/gatewayd                 本地 demo server
docs/shape-contract.md       设计口径
demo.sh                      本地对照脚本
```

## 构建与测试

```sh
go build ./...
go test ./...
```

## 本地 demo

`demo.sh` 会在本地端口（默认 18080，可用 `PORT` 覆盖）启动 `gatewayd`，
用若干条 `curl` 请求打出状态码与响应体，然后关闭服务并删除构建出的二进制：

```sh
bash demo.sh
```

也可以自己启动：

```sh
go run ./cmd/gatewayd -addr 127.0.0.1:8080
curl -X POST http://127.0.0.1:8080/account.create \
  --data 'username=ada&display_name=Ada&owner.email=ada%40example.com&enabled=true&attempts=3'
```
