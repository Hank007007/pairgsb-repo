# filterweave

员工目录查询入口前的过滤条件拼装件。调用方给出 `(属性, 操作符, 值)` 与组合方式，
本库负责属性名判定、值转义、布尔组合与长度检查，输出交给下层目录服务。

## 文档

- `docs/filter-spec.md` —— 对外行为规范，含条款编号。
- `docs/schema.md` —— 属性白名单。
- `DESIGN.md` —— 模块划分与各入口的预期行为。

## 安装与测试

只依赖标准库与 pytest。

```bash
python -m pytest -q -p no:cacheprovider
```

无安装步骤，测试通过 `pyproject.toml` 中的 `pythonpath` 直接导入包。

## 用法

```python
from filterweave import build_and, build_or, build_present

build_and([("uid", "=", "u1001"), ("departmentNumber", "=", "D-0042")])
# '(&(uid=u1001)(departmentNumber=D-0042))'

build_or([("mail", "=", "a@example.com"), ("mail", "=", "b@example.com")])
build_present("mobile")
# '(mobile=*)'
```

值不需要也不应该由调用方预先转义；属性名必须是 `docs/schema.md` 中列出的名称。

## 布局

```
filterweave/
├── __init__.py
├── build.py      # build_and / build_or / build_present / MAX_FILTER_LEN
├── escape.py     # escape_value / escape_dn_value
├── schema.py     # KNOWN / is_known_attr
└── errors.py
tests/
docs/
```
