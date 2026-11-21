# frameseal

自研网关的 WebSocket 前端解码器：按 `docs/frame-rules.md` 收帧、重组分片、解释控制帧、处理 permessage-deflate，并把消息交给业务 handler。

## 文档

- [`docs/frame-rules.md`](docs/frame-rules.md) — 收帧协议口径（帧头、mask、RSV、分片、控制帧、close、UTF-8、permessage-deflate、资源上限）
- [`docs/design.md`](docs/design.md) — 模块划分、数据流与各环节的职责边界
- [`docs/incident-reports.md`](docs/incident-reports.md) — 与帧解码相关的线上事故记录

## 目录

```
src/            解码器实现
  frame.js      帧头解析、长度、mask、payload
  fragment.js   分片重组与消息边界
  control.js    ping / pong / close
  permessage.js permessage-deflate 协商与解压
  text.js       文本消息校验
  conn.js       连接状态机
  errors.js     错误类型与关闭码
  limits.js     资源上限
  opcodes.js    操作码
  index.js      对外导出
test/           node:test 用例（内存 Buffer，无 socket）
testlib/        测试用的帧构造工具
docs/           协议与设计文档
```

## 运行

要求 Node.js 20 及以上，仅使用标准库。

```sh
node --test
```

## 用法示例

```js
const { Connection } = require("./src");

const conn = new Connection({
  extensionsHeader: req.headers["sec-websocket-extensions"] ?? "",
  onMessage: ({ type, data }) => {
    // type: "text" | "binary"
  },
  onPing: ({ payload }) => {
    // 在真实连接上回送同 payload 的 pong
  },
  onClose: ({ code, reason, byPeer, error }) => {
    // 记录关闭原因
  },
});

socket.on("data", (chunk) => {
  try {
    conn.feed(chunk);
  } catch (error) {
    // error.closeCode 是要写回对端的关闭码
  }
});
```
