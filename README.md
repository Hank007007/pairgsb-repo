# cborveil

设备端 OTA 升级代理使用的 CBOR manifest 解析库。

厂商服务端把一次升级包描述为一个 CBOR map，设备收到后依次解析、验签、按机型
选择分片。这个库是设备上唯一处理来路不明字节的地方。

## 目录

```
docs/manifest-cbor.md     manifest 的 CBOR 编码规范
docs/incident-reports.md  已归档的解析相关事故
DESIGN.md                 各模块的职责与目标行为说明
internal/cbor/            通用 CBOR 子集：解码、规范化判定、编码
internal/manifest/        manifest 映射、待签字节、验签、目标选择
cmd/otadump/              命令行 dump 工具
```

## 构建与测试

```
go build ./...
go test ./...
```

只依赖 Go 标准库。

## otadump

```
go run ./cmd/otadump internal/manifest/testdata/example.cbor
go run ./cmd/otadump -model P1 internal/manifest/testdata/example.cbor
```

输出为稳定的逐行格式，解析失败时向 stderr 报错并以非零状态退出。

## 使用

```go
data, _ := os.ReadFile(path)
m, err := manifest.Parse(data)
if err != nil {
    return err
}
if err := manifest.Verify(m, vendorPublicKey); err != nil {
    return err
}
target, err := manifest.Select(m, device.Model)
if err != nil {
    return err
}
```

## 规范要点

- 只接受 manifest-cbor.md 第 2 节列出的 CBOR 类型。
- manifest 必须整体为规范化编码：map 键按编码字节排序、最短形式整数、
  不允许 indefinite-length。
- 签名覆盖除 `sig`、`key_id` 之外的全部字段，待签字节与键顺序无关。
- 单 manifest 上限 1 MiB，嵌套深度上限 16。
- 机型匹配必须完整相等。
