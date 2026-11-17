# decframe

内部 RPC 帧解码库：把边缘节点发来的二进制帧还原成 `Frame`。

```go
frame, err := wire.Decode(buf)
if err != nil {
    // 按错误码处理畸形帧
}
```

- 线格式与硬性约束：[docs/frame-format.md](docs/frame-format.md)
- 结构说明与设计取舍：[docs/design.md](docs/design.md)

## 开发

```sh
go test ./...
```

只依赖 Go 标准库。
