# tuplegrate 设计说明

本文说明 tuplegrate 的总体结构、数据流与需要维持的性质。规范性语义以
[relation-model.md](relation-model.md) 为准；本文不描述具体实现。

## 1. 目标

tuplegrate 为内部服务提供统一的细粒度判权能力：

- 授权数据以 `object#relation@subject` 三元组表达，写入与判定解耦；
- 权限规则集中在 schema 中维护，服务进程不硬编码授权逻辑；
- 判定入口单一，调用方只需给出 subject、relation、object 和快照版本；
- 判定结果可复现：相同 revision 上的判定在快照可用期间必须稳定。

## 2. 分层

```
调用方
  │  Check(subject, relation, object, revision)
  ▼
internal/check      rewrite 求值、深度预算、快照选择、错误分类
  │                 │
  │                 └── internal/schema     schema 文本 → rewrite 树
  ▼
internal/subject    subject 覆盖判定（精确、通配、subject set 展开入口）
  │
  ▼
internal/tuple      三元组解析、版本化存储、按对象/关系索引、快照与 GC 水位
```

### 2.1 internal/tuple

负责数据面的全部内容：

- `Object`、`Subject` 的解析与规范化表示；
- 按 `(object, relation)` 建立索引，供 `self` 与 `from` 读取；
- 单调 revision、历史记录、快照视图与 GC 水位；
- 幂等写入与删除语义。

存储在内存中完成，对外暴露只读视图接口。快照一旦取得，后续写入不得影响其
内容。

### 2.2 internal/schema

把 schema 文本编译为不可变的 rewrite 树。编译阶段完成：

- 词法与语法分析；
- 运算符优先级与结合性；
- 重复类型、重复关系、未闭合声明等结构错误的报告；
- 关系名到 rewrite 节点的索引。

schema 不感知具体 tuple，也不做运行时判定。

### 2.3 internal/subject

集中实现 subject 覆盖规则。该模块只回答「存储中的某个 subject 是否覆盖请求
subject」，不负责展开 subject set 的后续求值；展开由 check 层发起并受深度
预算约束。

### 2.4 internal/check

判定的编排层：

1. 校验请求；
2. 依据 revision 选择快照视图；
3. 在 schema 中解析目标关系；
4. 按 rewrite 语义求值；
5. 维护每条路径的深度预算；
6. 把内部错误映射为对外错误分类。

## 3. 求值模型

rewrite 树是一棵布尔表达式树。求值时携带三个上下文：

- **视图**：某 revision 的只读 tuple 索引；
- **坐标**：当前对象与当前关系名；
- **预算**：当前路径还允许的下推次数。

`self` 从视图读取当前坐标下的 tuple；关系引用在同一对象上换关系；`from`
先把 tuple subject 解释为目标对象，再在目标对象上按左侧关系继续求值。
subject set 的展开等价于一次关系引用。

预算随路径复制，不跨分支共享。这样并集、交集、差集的不同操作数互不干扰，
而每一条实际求值路径仍然有界。

## 4. 快照

### 4.1 版本来源

存以操作日志的形式保留变更。视图的构造方式是：从空状态开始，按顺序重放
revision 不超过目标值的操作。因此任意未被 GC 的 revision 都可以单独重建。
实现可以在空间紧张时对日志做压缩，但对外语义不变。

### 4.2 与 Check 的衔接

`Check` 的 revision 是视图选择的唯一依据：

- `0` → 当前视图；
- 正数 → 对应版本视图；
- 不可用版本 → 显式错误。

返回值中的 revision 来自实际使用的视图，而不是存储的最新版本。

### 4.3 GC 与水位

GC 只推进可见性水位，不改变当前视图、不改变 revision 编号。严格低于水位的
版本视为已回收。判定入口必须拒绝这些版本，不能以最新视图替代。

## 5. 错误分类

| 错误 | 含义 |
| --- | --- |
| `invalid_request` | subject、object 或 relation 无法解析 |
| `unknown_relation` | 对象命名空间或关系不在 schema 中 |
| `depth_exceeded` | 单条求值路径超过 25 次下推 |
| `stale_revision` | 请求版本低于 GC 水位 |
| `future_revision` | 请求版本高于存储当前版本 |

错误与「拒绝」是不同概念。版本错误、深度错误必须显式返回，不能被转换为
`allowed = false`，否则调用方无法区分「没有权限」与判定失败」。

## 6. 需要维持的性质

1. **授权单调性由规则决定**：差集只能收窄，不能放大。`allowed` 为真必须能
   追溯到并集、交集、`self`、关系引用或 `from` 的正向证据。
2. **subject 类别隔离**：通配授权面向终端 subject，subject set 不会被通配
   命中。
3. **有界求值**：任意 tuple 图与 schema 引用关系都必须在深度预算内结束。
4. **快照确定性**：相同 revision 上的判定在快照可用期间结果一致。
5. **错误透明**：版本不可用与深度耗尽必须让调用方可见。

## 7. 测试策略

单元测试按包划分：

- tuple：解析、索引、幂等、revision 推进、快照重放、GC 水位；
- schema：语法、优先级、`from` 编译结果、结构错误；
- subject：精确匹配与通配匹配；
- check：并集、交集、subject set 展开、浅层嵌套、请求校验、结果版本。

集成场景把多个包串起来，覆盖一条写入到判定的完整路径。边界语义（深链、
环、差集、跨对象跳转、旧版本读取、subject set 通配）由规范定义，测试可以
按规范逐条构造。
