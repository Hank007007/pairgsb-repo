# 变更记录

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 的格式。

## [Unreleased]

### 新增

- `LeaseManager.owner_of(resource)`，查询资源当前持有者。

### 变更

- `LeaseManager.close()` 现在会把底层后端连接一并释放。

### 修复

- 修复并发 `acquire` 同一资源时两个客户端可能同时抢占成功的问题：`acquire` / `renew` / `release` 原先的「先查后写」不是原子操作，现改为通过后端的原子比较并写入（`set_if_absent` / `expire_if_owner` / `delete_if_owner`）完成；未实现这些可选方法的旧后端仍可继续使用。

## [0.3.0] - 2026-08-02

### 新增

- `Backend` 协议与 `RedisBackend` 实现，`LeaseManager` 不再直接依赖 `redis` 客户端。
- `LeaseManager(default_ttl_ms=..., timeout_s=...)` 两个构造参数。

### 变更

- `Lease.acquired_at` 由 wall clock 改为 `time.monotonic()`，不再受系统时钟调整影响。

### 移除

- 构造参数 `clock`，改由 `Backend` 承担时间语义。

## [0.2.0] - 2026-06-14

### 新增

- 首次公开发布：`acquire` / `renew` / `release`。

[Unreleased]: https://example.invalid/leasekit/compare/v0.3.0...HEAD
[0.3.0]: https://example.invalid/leasekit/compare/v0.2.0...v0.3.0
[0.2.0]: https://example.invalid/leasekit/releases/tag/v0.2.0
