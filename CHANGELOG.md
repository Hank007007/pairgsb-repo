# 变更记录

格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)。

## [Unreleased]

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
