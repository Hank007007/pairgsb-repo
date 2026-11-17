# credlint

`credlint` 是提交前运行的凭据扫描器：给定目录或 unified diff，按行切开后用规则表加熵值找出疑似密钥，命中值打码后写入报告。报告可以给人看，也可以 `--format json` 挂到 CI 上，用 `--fail-on-findings` 挡住合并。

## 安装与运行

只依赖标准库；测试需要 pytest。

```bash
python -m credlint path/to/repo
python -m credlint --diff changes.diff --fail-on-findings
python -m credlint --format json --ignore .credlintignore .
```

## 目录结构

```
credlint/
  walker.py      枚举候选文件、读取文本、判定二进制
  rules.py       规则表加载与编译
  entropy.py     随机性打分
  decode.py      单层 base64 / URL-safe base64 解码
  ignorefile.py  allowlist 解析与路径匹配
  diff.py        unified diff 解析
  scanner.py     规则 + 熵值 + 解码的组合
  report.py      文本 / JSON 报告
  redact.py      打码，供报告与日志管道复用（也可单独 import）
  cli.py         命令行入口
rules/
  default.json   规则表
tests/           回归测试
docs/
  scan-rules.md  扫描口径（规范）
  design.md      设计与模块职责
```

## 规则表

`rules/default.json` 是 JSON 文档，`rules` 数组里每条规则包含：

| 字段 | 含义 |
|---|---|
| `id` | 规则标识，出现在报告里 |
| `pattern` | 正则表达式 |
| `group` | 保存凭据的捕获组编号 |
| `entropy` | 是否还要求命中值通过熵值判定 |
| `description` | 给人看的说明 |

新增规则只需编辑 JSON；`--rules` 可以指向另一份表。

## allowlist

`.credlintignore` 每行一个路径模式，`#` 开头为注释，空行忽略。模式相对于扫描根目录、用 `/` 分隔。匹配语义见 `docs/scan-rules.md`。

## redact 子模块

`credlint.redact` 可以脱离扫描器单独使用，业务方的日志管道直接调用：

```python
from credlint.redact import redact_text, redact_url

safe_url = redact_url(request_url)       # 处理 URL
safe_line = redact_text(log_line)        # 处理整行日志
```

## 退出码

| 退出码 | 含义 |
|---|---|
| 0 | 扫描完成，未发现（或未启用 `--fail-on-findings`） |
| 1 | 有命中且启用了 `--fail-on-findings` |

## 测试

```bash
python -m pytest -q -p no:cacheprovider
```

行为口径以 `docs/scan-rules.md` 为准。
