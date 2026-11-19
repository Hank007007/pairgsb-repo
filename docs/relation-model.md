# 关系模型规范

本文件是 tuplegrate 的规范性说明。实现与本文不一致时，以本文为准。文中使用
`MUST` / `MUST NOT` 表达强制性要求。

## 1. 数据模型

### 1.1 Object

对象由命名空间和标识符组成，写作 `namespace:id`。命名空间的字符集为
`[A-Za-z_][A-Za-z0-9_]*`，标识符为非空字符串。对象自身不携带关系。

### 1.2 Subject

subject 有三种形态：

| 形态 | 示例 | 含义 |
| --- | --- | --- |
| 终端 subject | `user:alice` | 单个用户或服务 |
| subject set | `group:eng#member` | 对象 `group:eng` 上 `member` 关系当前覆盖的全体 subject |
| 通配 subject | `user:*` | 命名空间 `user` 内的任意终端 subject |

subject set 的形式为 `namespace:id#relation`，其中 `relation` 非空。通配形式
只允许标识符为 `*` 且不带 `#relation`。

### 1.3 Tuple

授权数据由三元组构成：

```
object#relation@subject
```

一条 tuple 表示「`subject` 在 `object` 上拥有 `relation`」。例如
`document:doc-1#viewer@group:eng#member` 表示工程组的所有成员都是该文档的
查看者。

同一 `(object, relation, subject)` 在存储中至多存在一次，重复写入是幂等的。

### 1.4 Revision

存储维护单调递增的 revision，初值为 `0`。每一次实际改变存储内容的写入或删除
都会把 revision 加一。幂等写入、删除不存在的 tuple 不改变 revision。

## 2. Schema

### 2.1 语法

schema 是一组类型声明：

```ebnf
schema      = { typeDecl } ;
typeDecl    = "type" ident "{" { relationDecl } "}" ;
relationDecl = "relation" ident "=" expr ;
expr        = orExpr ;
orExpr      = andExpr { "|" andExpr } ;
andExpr     = notExpr { "&" notExpr } ;
notExpr     = term { "but" "not" term } ;
term        = "self"
            | ident [ "from" ident ]
            | "(" expr ")" ;
```

`ident` 的字符集为 `[A-Za-z_][A-Za-z0-9_-]*`。`type`、`relation`、`self`、
`from`、`but`、`not` 是保留字，不能用作类型名或关系名。

空白与注释（`//` 行注释、`/* ... */` 块注释）可出现在任意 token 之间。

### 2.2 优先级与结合性

从低到高依次为：`|`、`&`、`but not`。三者均为左结合。括号可以改变结合顺序。

例如：

```
a | b & c but not d
```

等价于：

```
a | (b & (c but not d))
```

### 2.3 关系定义

每个 `relation` 声明恰好对应一个 rewrite 表达式。同一类型内不允许重复声明
关系；同一 schema 内不允许重复声明类型。

关系名在求值时以「当前对象 + 关系名」为坐标解析。schema 中引用其他命名空间的
关系时不需要额外声明。

示例：

```
type folder {
    relation parent = self
    relation editor = self
    relation banned = self
    relation viewer = self | editor | viewer from parent
    relation commenter = viewer but not banned
}

type group {
    relation member = self
}
```

## 3. Rewrite 语义

对固定的快照、对象 `O`、关系 `R` 和请求 subject `S`，表达式求值得到布尔值。
`eval(O, R_expr, S)` 表示在对象 `O` 上对表达式求值；`rel(O, R)` 表示在对象
`O` 上对关系 `R` 的完整定义求值。

### 3.1 self

`self` 的值为真，当且仅当对象 `O` 上关系 `R` 的 tuple 集合中存在一个 subject
覆盖 `S`（覆盖规则见 §4）。`R` 是包含该 `self` 的当前关系名。

### 3.2 并集 `a | b`

任一操作数为真即为真。各操作数彼此独立，互不影响。

### 3.3 交集 `a & b`

所有操作数均为真才为真。

### 3.4 差集 `a but not b`

`a but not b` 的值为真，当且仅当：

1. `a` 的值为真；并且
2. `b` 的值为假。

`b` 的结果 MUST NOT 被并入结果。即使 `a` 为假、`b` 为真，结果也必须为假。
差集只用于从 `a` 中剔除 `b` 命中的 subject，不能扩大授权。

### 3.5 关系引用 `a`

单独的标识符 `a` 是 computed userset：其值为 `rel(O, a)`，即在同一对象上对
关系 `a` 求值。

### 3.6 Tuple-to-userset `a from b`

`a from b` 用于跨对象跳转：

1. 读取对象 `O` 上关系 `b` 的 tuple 集合；
2. 对其中每一条 tuple，取 subject 对应的对象 `T`：
   - 终端 subject `ns:id` 的对象为 `ns:id`；
   - subject set `ns:id#r` 的对象为 `ns:id`；
   - 通配 subject 不指向单个对象，MUST 被跳过；
3. 在对象 `T` 上对关系 `a` 求值；
4. 任一目标对象上 `a` 为真，整个表达式即为真。

`b` 只负责选择 tuple，不作为目标对象上的求值关系。上例中
`viewer from parent` 到达父对象后，求值的是父对象的 `viewer`，不是父对象的
`parent`。

`a` 与 `b` 可以同名。此时仍然是「用 `b` 选 tuple，到目标对象上求值 `a`」。

## 4. Subject 匹配

设存储中的 subject 为 `stored`，判定请求中的 subject 为 `requested`。

### 4.1 精确匹配

当 `stored` 与 `requested` 的命名空间、标识符、关系（若存在）都相同时，
二者匹配。

### 4.2 通配匹配

`stored` 为通配 subject `ns:*` 时，它匹配 `requested` 当且仅当：

- `requested` 是终端 subject（不带 `#relation`）；并且
- `requested` 的命名空间等于 `ns`。

通配 subject MUST NOT 匹配任何 subject set（如 `group:eng#member`），也
MUST NOT 匹配其他命名空间的终端 subject。

### 4.3 subject set 展开

`stored` 为 `ns:id#r` 时，它对 `requested` 的覆盖范围等同于
`rel(ns:id, r)` 对 `requested` 的判定结果。展开 subject set 属于递归求值，
消耗深度预算（§5）。

### 4.4 通配 subject 作为请求 subject

判定请求可以携带 subject set 作为 `requested`。此时只有精确匹配可以命中，
通配 subject 不能命中该请求。

## 5. 递归深度

### 5.1 上限

单条求值路径允许的最大下推深度为 **25**。一次「下推」指：

- 展开一个 subject set；
- 对 `a` 形式的关系引用求值；
- 执行一次 `a from b` 跳转。

顶层 `Check` 指定的关系不计入推次数。第 26 次下推必须返回
`depth_exceeded` 错误，不得继续求值。

### 5.2 路径语义

深度按求值路径计算。同一表达式中不同分支各自拥有独立的预算，一个分支的失败
不得消耗其他分支的额度。差集、并集、交集的不同操作数从相同的剩余深度开始。

### 5.3 必须有界

任意 tuple 图上的求值都必须在有限步内返回。包括：

- 长度超过 25 的 `parent` 链：必须在 25 次下推以内返回 `depth_exceeded`，
  不得一直求值到链尾；
- 互相引用形成的环（例如 `A` 的 parent 是 `B`、`B` 的 parent 是 `A`）：
  必须在 25 次下推以内返回 `depth_exceeded`，不得阻塞、不得栈溢出；
- schema 内关系互相引用形成的环同样受此预算约束。

## 6. 快照、Revision 与 GC

### 6.1 判定所依据的视图

`Check` 请求的 `revision` 字段决定本次判定使用的 tuple 视图：

- `revision == 0`：使用当前最新视图；
- `revision == R > 0`：使用恰好为 `R` 的视图，即只包含 revision `<= R` 的
  写入，且不包含 revision `> R` 的写入。

返回值中的 `revision` MUST 是本次判定实际使用的视图版本。

### 6.2 版本错误

- 请求的 revision 大于存储当前 revision：返回 `future_revision`。
- 请求的 revision 低于存储已回收（GC）水位：返回 `stale_revision`。
- 上述两种情况 MUST NOT 回退到最新视图继续判定。

### 6.3 GC 水位

存储提供 GC 操作，将水位推进到某个 revision。水位以下的快照视为已不可用。
水位本身在数据仍可重建时可用；严格低于水位的请求必须返回 `stale_revision`。

GC MUST NOT 改变当前视图，也 MUST NOT 改变 revision 编号。

### 6.4 快照隔离

版本 `R` 的视图在创建后不随后续写入变化。对同一 revision 的重复判定必须得到
相同结果，除非该 revision 已被 GC。

## 7. Check API

### 7.1 请求

```
Check(subject, relation, object, revision)
```

- `subject`：请求主体，允许终端 subject 或 subject set；
- `relation`：目标关系名；
- `object`：目标对象；
- `revision`：快照版本，`0` 表示最新。

### 7.2 结果

成功时返回：

- `allowed`：布尔判定结果；
- `revision`：实际使用的视图版本。

对象或关系在 schema 中不存在时返回 `unknown_relation`。请求格式非法时返回
`invalid_request`。递归超出预算时返回 `depth_exceeded`。

### 7.3 判定顺序

1. 解析 subject 与 object；
2. 解析 revision 对应视图（§6）；
3. 在 schema 中定位对象命名空间下的关系；
4. 在该视图上求值 rewrite 表达式（§3、§4、§5）。

任何一步失败都不得返回 `allowed = true`。

## 8. 一致性要求

- 判定只读取请求 revision 对应的视图，不读取「当前」存储的其他部分。
- 并集、交集、差集和 `from` 的组合结果必须与逐个操作数的定义一致。
- subject 匹配必须区分终端 subject 与 subject set（§4.2）。
- 深度预算对每一条求值路径独立生效（§5.2）。
- 错误必须显式返回，不得用 `allowed = false` 掩盖版本错误或深度错误。
