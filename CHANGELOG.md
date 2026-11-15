# 变更记录

## [Unreleased]

## [2.3.0] - 2026-10-21

### Added

- 分段写入：`Journal#append` 按 `maxEntries` 自动切段
- `verifyChain` 返回每段的结论与整体 ok
- CLI：`node src/cli.js verify <目录>`

## [2.2.0] - 2026-09-15

### Added

- 记录级 HMAC，密钥从 `CHAINLOG_KEY` 读
- 规范化编码（`canonicalize`）

## [2.1.0] - 2026-08-08

### Added

- 初版追加日志与顺序读取
