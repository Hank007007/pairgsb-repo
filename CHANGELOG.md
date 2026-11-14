# 变更记录

格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)。

## [Unreleased]

### Fixed

- 验签改为对原始请求体字节计算 SHA-256，不再先按 UTF-8 解码再编码，
  修复非 UTF-8 正文（如含加密流的 PDF）全部 401 的问题
- `X-Signature` 头名按 HTTP 语义大小写不敏感匹配，
  修复网关把头名归一化成 `X-SIGNATURE` 后全部 401 的问题
- 验签遍历密钥环内所有未过期密钥，轮换期间旧密钥签出的请求不再被误判 `bad_signature`
- 待签名字符串按规范修正为五行，去掉多出的空行，
  修复与按规范实现的对端（Go）签名值永远不一致的问题
- 签名比较改用 `hmac.compare_digest` 常量时间比较，消除时序侧信道
- 防重放缓存键改为 `t + v1` 组合，同一秒内的不同事件不再被误判 `replay`；
  命中重放按规范返回幂等成功（`ok=True, reason="replay"`）
- `ReplayCache.check_and_store` 的检查与写入合并到同一把锁内，并发下不再双双放行
- `DeliveryStore.record` 改用 `INSERT OR IGNORE` 原子写入，
  并发重复投递只成功一次且不再抛 `sqlite3.IntegrityError`；
  改为持有单条连接，修复 `DeliveryStore(":memory:")` 报 `no such table: deliveries`

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
