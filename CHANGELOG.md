# Changelog

本文件记录 filtergate-doc-query 的变更。格式参考 Keep a Changelog。

## Unreleased

### Security

- 字段值层改为算子正向白名单：`$where`、`$expr`、`$function`、`$accumulator`、
  `$text`、`$jsonSchema` 及任何白名单外的 `$` 算子一律返回
  `UNKNOWN_OPERATOR`，不再透传至 adapter。
- 租户隔离改为结构保证：解析期丢弃任意层级的用户 `tenant_id` 条件，编译后由
  网关在根 `and` 注入唯一的会话租户子句，用户输入无法覆盖、追加或削弱。
- 解析阶段执行资源上限：逻辑嵌套深度上限 32（递归进入子节点前检查，
  返回 `DEPTH_EXCEEDED`，不再触发 `RecursionError`）、`$in` 长度上限 100
  （`LIST_TOO_LONG`）、`$regex` pattern 上限 200 字符且禁止 NUL。
- `$regex` 按字面量语义下发，adapter 翻译时用 `re.escape` 转义正则元字符；
  `$options` 仅允许不重复的 `i` / `m`，其余返回 `REGEX_INVALID`，单独出现
  返回 `INVALID_FILTER`。
- 字段白名单改为全名精确匹配，含 `.` 或 `$` 的字段名返回
  `FIELD_NOT_ALLOWED`，不再按顶层字段放行子文档路径。
- 类型校验不再隐式转换：整数字段拒绝数字字符串、布尔与浮点，`$in` 必须是非空
  数组且元素与字段类型逐一匹配，类型不符统一返回 `TYPE_MISMATCH`。

### Added

- 初始版本：filter DSL 解析、类型规范化、租户作用域注入与 Mongo 风格 adapter。
- `FilterGate.compile()` 公开契约、`CompileResult` 与 `ErrorCode` 错误码枚举。
- 端到端示例 `examples/query_demo.py` 与基线快照 `tests/snapshots/demo_query.json`。

## 0.1.0

- 首个可运行版本。
