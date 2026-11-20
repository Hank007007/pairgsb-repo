# privpath

`privpath` 是权限配置的变更审计器：读入变更前后的两份访问配置，输出「这次变更让谁多拿到了什么」，供 CI 卡口使用。

## 用法

```bash
python -m privpath.cli examples/before.json examples/after.json \
    --before-label main --after-label pr
```

退出码：报告中有 high 条目时为 `1`，配置错误为 `2`，否则为 `0`。

## 配置

一份配置是 JSON 对象，四段：`roles`、`inherits`、`bindings`、`principals`。字段含义与完整语义见 [`docs/diff-semantics.md`](docs/diff-semantics.md)；
一次真实事故的复盘见 [`docs/incident-reports.md`](docs/incident-reports.md)。

## 开发

```bash
python -m pytest -q -p no:cacheprovider
```

只依赖标准库与 pytest。
