# pyscan

一个只跑 AST 的 Python 污点分析器，目前带一条命令注入规则。

## 结构

| 文件 | 职责 |
|---|---|
| `pyscan/model.py` | `Rule` / `SinkSpec` / `SanitizerSpec` / `Finding` |
| `pyscan/engine.py` | 遍历 AST、传污点、报命中 |
| `pyscan/rules.py` | 内置规则 `DEFAULT_RULE` |
| `pyscan/__main__.py` | `python -m pyscan <路径>` |
| `docs/rule-semantics.md` | **引擎语义，以它为准** |
| `docs/feedback-notes.md` | 内部群里的漏报反馈 |

## 用法

```bash
python -m pyscan ./some/package
pytest -q
```

结果是 JSONL，一行一条 `{path, line, col, rule, sink, trace}`；
有命中时退出码 1，没有命中退出码 0。

## 依赖

纯标准库，Python 3.10+，`pytest` 只在开发时用。
