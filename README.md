# filtergate-doc-query

面向多租户日志检索的 filter DSL 网关。前端提交一段 JSON filter，网关按
[`docs/filter-grammar.md`](docs/filter-grammar.md) 完成校验，再由后端 adapter
翻译成查询文档下发。仓库只依赖 Python 标准库，测试使用 pytest。

## 目录结构

```
filtergate/
  __init__.py        公开导出
  gate.py            FilterGate.compile() 与 CompileResult
  grammar.py         filter DSL 解析
  coerce.py          操作数类型规范化
  scope.py           租户作用域注入
  fields.py          字段白名单与类型目录
  errors.py          错误码
  adapters/
    base.py          adapter 协议
    mongo.py         Mongo 风格 adapter
docs/
  filter-grammar.md  语法与安全规范（冻结）
  design.md          设计说明
examples/
  query_demo.py      端到端示例
tests/
  test_baseline.py   公开契约与正常路径用例
  snapshots/         基线查询快照
```

## 运行

```bash
python -m pytest -q -p no:cacheprovider
python examples/query_demo.py
```

`FilterGate.compile(filter, tenant_id)` 返回 `CompileResult`：

- 成功：`ok=True`，`query` 为后端查询文档，`error=None`；
- 失败：`ok=False`，`query=None`，`error` 为 `ErrorCode` 之一。

错误码是公开契约，只增不改。
