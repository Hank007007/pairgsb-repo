# condgrate 策略模型

本文是 condgrate 授权决策点（PDP）的行为规范。所有实现与测试以本文为准；文中描述的是系统**应当**表现出的行为，不涉及具体实现方式。

## §1 请求与属性模型

一次授权请求由四部分组成：

- **主体（principal）**：`role`、`dept`、`clearance`、`groups`、`mfa` 等属性。
- **资源（resource）**：`owner`、`classification`、`tags`、`tenant`、`type` 等属性。
- **动作（action）**：单个动作名，例如 `read`、`write`、`delete`。
- **上下文（context）**：`time`、`ip`、`tenant` 等运行时信息。

条件表达式中的属性引用使用点号路径，路径首段是属性来源：

```
principal.role
principal.clearance
resource.tags.env
resource.tenant
context.ip
action
```

`action` 是标量，其余来源的路径逐段在嵌套对象中取值。属性来源只限于 `principal`、`resource`、`context`、`action` 四者。

## §2 属性类型与比较前转换

每个属性在目录中声明类型，类型取值为：

| 声明类型 | 可接受的值 |
|---|---|
| `string` | 字符串 |
| `number` | 数字，或可完整解析为数字的字符串 |
| `bool` | 布尔值，或字符串 `"true"` / `"false"` |
| `list` | 数组（元素按字符串处理） |

比较运算的两侧在比较前必须先按属性声明的类型完成转换，再做同类型比较：

- 两侧都是属性引用时，各自的声明类型必须一致；一侧为字面量时，以属性侧的声明类型为目标类型转换字面量；两侧都是字面量时按 JSON 原始类型比较。
- `number` 类型按数值大小比较，`>`、`<`、`>=`、`<=` 均为数值序，不得按字符串字典序。
- `string` 类型按字符串逐字符比较，大小写敏感。
- `bool` 类型的 `eq` / `ne` 按真值比较，`"true"` 与 `true` 在转换后相等；`>`、`<` 等大小比较对 `bool` 无定义。
- 无法完成转换（例如把 `"high"` 声明为 `number`，或对 `bool` 做大小比较）时，该比较的结果为 indeterminate（§3），不得猜测为 true 或 false。

例：属性 `principal.clearance` 声明为 `number`，值为字符串 `"10"`，条件 `principal.clearance >= 3` 转换后比较 `10 >= 3`，结果为 true；值为 `"9"` 时转换后为 `9 >= 3`，结果同样为 true。若按字符串字典序比较 `"10"` 与 `"3"` 会得到错误结论，这是被明确禁止的。

## §3 缺属性与三值结果

条件求值的结果有三种：`true`、`false`、`indeterminate`。

- **缺属性**：路径在属性来源中不存在时，引用该路径的比较、`in`、`contains` 结果为 `indeterminate`。`present` 是例外：路径不存在时它回答 `false`，因为“是否存在”本身是可判定的。
- **类型不匹配**：值无法按声明类型解释，或运算对该类型无定义时，结果为 `indeterminate`。
- **表达式错误**：未知运算、格式错误、求值过程中出现无法处理的值时，结果为 `indeterminate`。

三值逻辑组合规则：

- `and`：任一操作数为 `false` 则 `false`；否则若任一为 `indeterminate` 则 `indeterminate`；否则 `true`。
- `or`：任一操作数为 `true` 则 `true`；否则若任一为 `indeterminate` 则 `indeterminate`；否则 `false`。
- `not`：`true` ↔ `false`，`indeterminate` 取反仍为 `indeterminate`。

`indeterminate` 不得被当作 `true` 或 `false` 参与最终授权结论，其处理方式见 §10。

## §4 条件表达式

条件表达式是一棵 JSON 树，节点如下：

| 节点 | 形式 | 语义 |
|---|---|---|
| `and` | `{"and": [expr, ...]}` | 合取，三值逻辑 |
| `or` | `{"or": [expr, ...]}` | 析取，三值逻辑 |
| `not` | `{"not": expr}` | 取反，三值逻辑 |
| `cmp` | `{"cmp": {"op": "eq\|ne\|lt\|lte\|gt\|gte", "left": ref, "right": ref}}` | 按 §2 转换后比较 |
| `in` | `{"in": {"path": "principal.groups", "values": [ref, ...]}}` | 成员判断 |
| `contains` | `{"contains": {"path": "resource.tags", "value": ref}}` | 集合/字符串包含 |
| `present` | `{"present": {"path": "principal.mfa"}}` | 属性是否存在 |

`ref` 有两种写法：

- 属性引用：`{"attr": "principal.role"}`；
- 字面量：直接写 JSON 值，例如 `"admin"`、`3`、`true`、`["a","b"]`。

`in`、`contains`、`present` 的 `path` 是点号路径字符串，路径可以跨多段，例如 `resource.tags.env`。

各运算的细则：

- **`cmp`**：按 §2 转换后比较；类型不匹配或转换失败为 `indeterminate`；引用缺失属性为 `indeterminate`。
- **`in`**：
  - `path` 解析为数组时，将数组元素逐个与 `values` 中任一项比较（同样先按 §2 转换，元素与字面量比较时按元素的自然类型），命中任一即 `true`。
  - `path` 解析为单个字符串时，**只有与某个候选值完全相等**才算命中；`"admin" in ["super-admin"]` 为 `false`，不得使用子串或前缀匹配。
  - `values` 中的模式支持通配：`*` 匹配**恰好一个**以 `:` 分隔的段。`tenant:*` 命中 `tenant:eu`，不命中 `tenant:eu:finance`；`tenant:*:*` 可命中 `tenant:eu:finance`。
  - `path` 无法解析或属性缺失时为 `indeterminate`，不得默认命中。
- **`contains`**：
  - 左侧为数组时，判断是否有元素与右侧值相等。
  - 左侧为字符串时，判断右侧字符串是否为左侧的子串。
  - 其他类型为 `indeterminate`。
  - 缺失属性为 `indeterminate`。
- **`present`**：路径存在且值非 null 为 `true`，否则 `false`；不会返回 `indeterminate`。

## §5 策略结构与 target 匹配

一条策略的 JSON 形如：

```json
{
  "id": "org-deny-contractors",
  "effect": "deny",
  "principal": {"role": ["contractor"], "dept": ["*"]},
  "action": ["read", "write"],
  "resource": {"type": ["*"], "classification": ["internal", "confidential"]},
  "condition": {"present": {"path": "principal.mfa"}},
  "obligations": ["notify-security"]
}
```

- `principal`、`resource` 两个匹配块为“属性名 → 候选值列表”的映射。块内**每个**键都必须与请求中的对应属性匹配；块间为逻辑与。
- `action` 是一个候选动作列表，请求动作命中其中任一项即可；列表为空视为不限制。
- 候选值可以是标量或数组，解析后统一为字符串列表。
- 候选值为 `"*"` 时匹配任意值。`*` 按 §4 的分段语义解释：它匹配恰好一个 `:` 分隔段；对不含 `:` 的属性（例如 `resource.type`）这等价于任意值，覆盖该属性的整张取值表。
- 候选值中的 `*` 是分段通配，语义同 §4：每个 `*` 匹配恰好一个 `:` 分隔段。`tenant:*` 命中 `tenant:eu`，不命中 `tenant:eu:finance`。
- 请求中缺少 target 要求的属性时，该 target 不匹配；target 匹配不使用三值逻辑，只有匹配 / 不匹配两种结果。
- `condition` 可省略；省略等价于恒为 `true`。
- `effect` 只能是 `permit` 或 `deny`。

## §6 策略收集

策略按 `org → team → project` 三层挂载。一次求值必须：

1. 从最具体的 `project` 层开始，向 `team`、`org` 逐层向上遍历；
2. 对**每一层**做 target 匹配，收集该层所有 target 命中的策略；
3. 收齐整条链上所有命中策略后再进入合并，**不得**因为某一层有命中就停止向上收集。

父层策略对子层的限制与子层策略同等参与后续合并。收集结果的顺序不影响最终结论。

## §7 结论合并

合并的输入是收集到的全部策略及其条件求值结果。

1. 只有条件求值为 `true` 的策略参与合并；`false` 的策略视为未命中；`indeterminate` 由 §10 处理。
2. 若任一命中策略的 effect 为 `deny`，最终结论为 `deny`。**deny 优先于 permit**，与策略所在层级、收集顺序、策略 ID 无关。
3. 否则，若存在 effect 为 `permit` 的命中策略，结论为 `permit`。
4. 否则（无命中策略），默认结论为 `deny`。

## §8 obligation 传播

- obligation 是 permit 附带的后置动作（例如“记录审计日志”“通知安全团队”）。
- obligation **只从 effect 为 `permit` 且条件为 `true` 的策略中收集**，按策略 ID 去重后随结论返回。
- 最终结论为 `deny` 时，不返回任何 obligation；被 deny 覆盖的 permit 所携带的 obligation 不得传给调用方。
- 最终结论为 `permit` 但没有任何 obligation 时，返回空列表。
- 默认拒绝（§7 第 4 条）没有 obligation。

## §9 属性来源与目录权威

属性有两个来源：

1. **目录（directory）**：由身份系统定期同步，带有属性声明类型，是属性的权威来源。
2. **请求体（request attributes）**：调用方随请求附带的属性提示。

优先规则：

- 目录中存在的属性，一律以目录值为准；请求体中的同名值必须被忽略，不得覆盖目录值。尤其是 `clearance`、`role`、`dept`、`groups`、`mfa` 等安全相关属性。
- 目录中不存在的属性，可以采用请求体中的值，并按 JSON 原始类型推断其类型。
- 两个来源都没有的路径视为缺失，求值按 §3 处理，不得返回零值冒充存在。
- 目录同步的数据结构由身份系统定义，PDP 只读，不写回。

## §10 决策结果与 indeterminate 处理

对外决策结果包含：结论（`permit` / `deny`）、原因、obligation 列表、命中的策略 ID 列表。

- 条件求值出现 `indeterminate` 时，**最终结论必须为 `deny`**，并在原因中说明是哪条策略、哪个表达式导致的 indeterminate。
- `indeterminate` 不得被提升为 `permit`，也不得被静默忽略后改用其他 permit 策略放行。
- 存在 `indeterminate` 时，不得返回任何 obligation。
- 正常命中时，原因为命中的策略说明；默认拒绝时，原因说明没有策略命中。
