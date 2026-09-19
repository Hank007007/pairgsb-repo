# 变更记录

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 的格式。

## [Unreleased]

### 新增

- `LeaseManager.owner_of(resource)`，查询资源当前持有者。

### 变更

- `LeaseManager.close()` 现在会把底层后端连接一并释放。

### 修复

- 并发抢占同一资源时可能出现两个客户端同时持有租约：`acquire` / `renew` /
  `release` 改为通过 `Backend` 新增的原子原语（`set_if_absent` /
  `expire_if_value` / `delete_if_value`）完成「检查 + 写入」，`RedisBackend`
  分别用 `SET ... NX PX` 和 Lua 脚本实现；未实现新方法的旧后端自动退化为
  原来的非原子路径，行为不变。

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
