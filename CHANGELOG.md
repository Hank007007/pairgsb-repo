# Changelog

本文件记录 filtergate-doc-query 的变更。格式参考 Keep a Changelog。

## Unreleased

### Added

- 初始版本：filter DSL 解析、类型规范化、租户作用域注入与 Mongo 风格 adapter。
- `FilterGate.compile()` 公开契约、`CompileResult` 与 `ErrorCode` 错误码枚举。
- 端到端示例 `examples/query_demo.py` 与基线快照 `tests/snapshots/demo_query.json`。

## 0.1.0

- 首个可运行版本。
