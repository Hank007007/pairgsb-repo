# headlace

通知平台的出站邮件报文组装层。上游只提供结构化的通知字段，headlace 负责把它们
拼成一份完整的 RFC 5322 + MIME 报文，再交给 `net/smtp` 投递。

```
internal/header   头值编码（RFC 2047）、折行、头块渲染
internal/addr     地址列表切分、显示名与地址规格格式化
internal/mime     boundary 分配、正文组装、附件 Content-Disposition
internal/smtp     Message 模型与 Compose 入口
docs/             报文规范、设计说明、事故复盘
```

## 用法

```go
raw, err := smtp.Compose(smtp.Message{
    From:    "Notifications <notify@example.com>",
    To:      "alice@example.com, bob@example.com",
    Subject: "Your order has shipped",
    Text:    "Hello,\r\n\r\nYour order is on its way.\r\n",
})
if err != nil {
    return err
}
// raw 可直接作为 net/smtp 的 DATA 内容
```

## 文档

- `docs/mail-headers.md` —— 报文必须满足的头部、地址、MIME 规则。
- `docs/design.md` —— 模块职责与设计意图。
- `docs/incident-reports.md` —— 两起相关事故的复盘。

## 开发

只依赖 Go 标准库。

```
go test ./...
```

`internal/smtp/testdata/plain_ascii.eml` 是纯 ASCII 常规通知的字节基线。若确实需要
更新基线，用 `go test ./internal/smtp -run TestComposePlainASCIIMatchesSnapshot -update`
重新生成，并核对差异。
