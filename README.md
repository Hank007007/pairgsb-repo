# hooksign

Webhook 请求签名与校验库。对接方按 [`docs/signature-v2.md`](docs/signature-v2.md)
计算 `X-Signature` 头，我们负责校验。

## 模块

| 文件 | 职责 |
|------|------|
| `hooksign/headers.py` | 解析 `X-Signature` 头 |
| `hooksign/canonical.py` | 拼待签名字符串、算正文摘要 |
| `hooksign/signing.py` | 出方向签名，生成头 |
| `hooksign/verify.py` | 入方向校验 |
| `hooksign/secrets.py` | 密钥环（支持轮换） |
| `hooksign/cache.py` | 防重放缓存（进程内） |
| `hooksign/store.py` | 投递记录（SQLite，幂等 + 审计） |
| `examples/` | 一个最小接收端 + 发送端 |

## 运行测试

只用标准库，没有第三方运行时依赖：

```bash
python -m pytest -q
```

（Python 3.10+）

## 跑一个端到端示例

```bash
echo "devsecret" > .secret
python examples/receiver.py --secret-file .secret --port 8080 &
python examples/sender.py --secret-file .secret --event-id evt_1
python examples/sender.py --secret-file .secret --event-id evt_1 --repeat 2
kill %1
```

## 已知背景

`docs/incident-reports.md` 是对接方与合作方这两周报上来的问题摘录，
只有现象没有分析。规范以 `docs/signature-v2.md` 为准。
