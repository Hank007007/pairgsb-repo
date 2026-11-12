# 变更记录

格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)。

## [Unreleased]

### Fixed

- 待签名字符串多了空行，按规范改回 `v2/METHOD/PATH/timestamp/digest` 五行，
  与按规范自行实现的对端可以互验（`canonical.py`）
- 校验时直接对原始请求体字节算 SHA-256，不再先做 UTF-8 解码再编码，
  非 UTF-8 正文（如 PDF）不再误判 `bad_signature`
- `X-Signature` 头名按 HTTP 语义大小写不敏感匹配
- 校验时遍历密钥环内所有未过期密钥，轮换期混用新旧密钥不再 401
- 签名比较改为 `hmac.compare_digest` 常量时间比较，消除时序侧信道
- 防重放键改为 `t + v1`，同一秒到达的不同事件不再被误判为 `replay`；
  `ReplayCache.check_and_store` 的检查与写入在同一把锁内完成
- `DeliveryStore.record` 改为 `INSERT OR IGNORE` 原子写入，并发重复投递只成功一次、
  不再抛 `sqlite3.IntegrityError`；`:memory:` 复用同一连接，不再报 `no such table`
- `SecretRing.primary()` 按规范取"最后添加且未过期"的密钥
- 示例接收端按规范第 7 节映射原因码到 HTTP 状态码，`replay` 返回 200 幂等命中

## [0.4.0] - 2026-10-20

### Added

- `X-Signature` v2 待签名字符串（`canonical.py`）
- 出方向签名 `signing.sign_header()`
- 入方向校验 `WebhookVerifier.verify()`，返回结构化 `VerificationResult`
- 密钥环 `SecretRing`，支持轮换期同时存在多把密钥
- 防重放缓存 `ReplayCache`（进程内）
- SQLite 投递记录 `DeliveryStore`
- 示例接收端与发送端（`examples/`）

## [0.3.0] - 2026-09-30

### Added

- 初版 `X-Signature` v1 校验，只支持单密钥、只支持 JSON 正文

### Removed

- 移除 v1 里对 `Content-MD5` 头的依赖
