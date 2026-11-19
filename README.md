# tuplegrate

tuplegrate 是一套面向内部服务的关系型判权库。它采用与 Zanzibar 相同的基本模型：

- 授权数据是 `object#relation@subject` 形式的三元组；
- 权限规则独立写在 schema 中，支持并集、交集、差集和跨对象跳转；
- 对外只有一个判定入口 `Check(subject, relation, object)`，返回判定结果与本次使用的快照版本号。

规范见 [docs/relation-model.md](docs/relation-model.md)，架构与不变量见
[docs/design.md](docs/design.md)，安全团队的测试记录见
[docs/incident-reports.md](docs/incident-reports.md)。

## 包结构

| 包 | 职责 |
| --- | --- |
| `internal/tuple` | 三元组的解析、版本化存储与按对象/关系建立的索引 |
| `internal/schema` | schema 文本的词法/语法分析与 rewrite 树构造 |
| `internal/subject` | subject 匹配规则（终端用户、subject set、通配符） |
| `internal/check` | rewrite 求值、深度预算、版本快照与对外 `Check` API |

## 快速开始

```go
func grantAndCheck() (check.Result, error) {
    rules, err := schema.Parse(`
type document {
  relation editor = self
  relation viewer = self | editor
}
`)
    if err != nil {
        return check.Result{}, err
    }

    store := tuple.NewStore()
    doc, err := tuple.ParseObject("document:doc-1")
    if err != nil {
        return check.Result{}, err
    }
    alice, err := tuple.ParseSubject("user:alice")
    if err != nil {
        return check.Result{}, err
    }
    if _, err := store.Write(doc, "editor", alice); err != nil {
        return check.Result{}, err
    }

    checker := check.New(store, rules)
    return checker.Check(check.Request{
        Subject:  "user:alice",
        Relation: "viewer",
        Object:   "document:doc-1",
    })
    // result.Allowed == true, result.Revision == 1
}
```

## 测试

```sh
go test ./...
```

本项目只依赖 Go 标准库。

## 版本语义

每次成功的 `Write`/`Delete` 都会推进存储版本号。`Check` 请求中的 `Revision`
为 `0` 时表示最新版本；否则判定必须固定在该版本的视图上进行。已低于 GC 水位的
版本必须显式返回 `stale_revision`，不得回退到最新视图。
