# privpath diff 语义规范

本文规定 `privpath` 在「输入变更前后的两份访问配置、输出一次变更让谁多拿到了什么」这件事上的正确语义。
实现必须逐条满足本文；本文只描述行为，不规定内部结构。

## §1 配置模型与加载

配置是一份 JSON 对象，固定四段：

```json
{
  "roles": {
    "reader": {"rules": [{"actions": ["read"], "resources": ["docs/*"]}]},
    "editor": {"rules": [{"actions": ["write"], "resources": ["docs/*"], "deny": false}]}
  },
  "inherits": {"editor": ["reader"]},
  "bindings": {"alice": ["editor"]},
  "principals": ["alice"]
}
```

- `roles`：对象，键是角色名，值是该角色的策略，`rules` 是规则列表，可以为空。
- 每条规则：
  - `actions`：非空字符数组；
  - `resources`：非空字符串数组；
  - `deny`：可选。缺省为 `false`；允许的取值是布尔值，或字符串 `"true"` / `"false"`（大小写不敏感）。字符串要按字面含义解释为布尔，不能按「非空串即真」处理。其它类型（数字、对象、数组、其它字符串）属于配置错误。
- `inherits`：对象，键是角色名，值是角色名数组，缺省为空。继承关系可以成环。
- `bindings`：对象，键是主体名，值是角色名数组。
- `principals`：主体名字符串数组。

引用完整性在加载阶段检查：`inherits` 的每个键和每个值、`bindings` 的每个值都必须是 `roles` 里声明过的角色。
任何悬空引用都必须让加载失败，错误信息要指出**具体位置**（例如 `inherits["editor"][1]`、`bindings["alice"][0]`）和未知角色名，不允许静默丢弃，也不允许把问题留给下游。

加载成功后对外返回的配置对象只承载原始结构（角色、规则、继承、绑定、主体清单），不预先做权限展开。

## §2 角色继承闭包

记 `a -> b` 表示 `a` 继承 `b`，规则从 `b` 流向 `a`。

一个角色的祖先集合是从它出发、沿 `inherits` 边可达的所有角色，**不含它自身**：

- 传递性：`a -> b -> c` 时，`c` 的规则必须到达 `a`，不能只展开一层。
- 环：`a -> b -> a` 是合法配置。闭包计算必须能终止，不得把进程打崩；结果中每个角色最多出现一次。
- 环上回到起点的角色不进入祖先集合（角色永远不算自己的祖先），但闭包里其余的节点都要在。
- 悬空角色名在 §1 已经被拒绝，闭包不会遇到未知角色；如果遇到，也绝不能把异常带进结果。
- 结果顺序按首次发现顺序确定，保证同一份配置多次运行得到同一份报告。

## §3 有效权限与动作层级

一条授权记作 `Grant(action, resource, source, path)`：

- `source` 是规则所属的角色；
- `path` 是从「主体被绑定的角色」出发、沿继承边到达 `source` 的角色链，包含两端。

`effective(config, r)` 对 `r` 及其祖先上的所有**允许**规则做展开，得到 `r` 的有效授权集合：

- 动作名大小写不敏感，统一规范化为小写后再比较与输出。
- 动作存在层级：`admin` 蕴含 `write`，`write` 蕴含 `read`。
  - 一条 `admin` 规则同时产生 `read`、`write`、`admin` 三条授权（资源相同）；
  - 一条 `write` 规则同时产生 `read`、`write`；
  - `read` 只产生自身。
  - 不在级里的动作名按原样（小写）作为独立动作，不做蕴含展开。
- 完全相同（动作、资源、source、path 都相同）的授权只保留一条。
- 不同路径到达的同一项权限（不同 source，或同一 source 经不同继承链）要分别保留：报告需要展示多条路径。
- 经过 §4 的 deny 过滤后仍然留下的授权，才是有效授权。

同一口径也用于收集 deny：`r` 及其祖先上所有 `deny` 规则展开后的 `(action, resource, path)` 是判断「deny 是否被移除」的依据。

## §4 deny 对 allow 的压制

deny 优先。对 `r` 的计算范围（`r` 及其祖先）中的任一 deny 规则 `d` 和任一 allow 规则 `a`：

- 当 `d` 的动作集合（按 §3 展开）覆盖 `a` 产生的动作，且 `d` 的资源模式覆盖 `a` 的资源模式（§5），则 `a` 产生的该动作授权被压掉，不得出现在 `effective(config, r)` 中。
- 距离不影响结果：祖先上的 deny 压后代上的 allow，本角色的 deny 也压继承来的 allow。
- deny 只压同一资源 / 同一动作，不波及别的资源或别的动作。
- deny 规则本身不产生授权；它也不出现在有效授权集合里。
- 如果存在多个 deny，任意一个覆盖即压掉。

## §5 资源模式与覆盖

- 资源串里只有 `*` 是元字符，匹配任意长度字符序列（包括 `/`）。
  - `*` 匹配一切资源；
  - `docs/*` 匹配 `docs/secret`、`docs/a/b`；
  - `docs/*.md` 匹配 `docs/x.md`、`docs/a/b.md`。
- `matches(pattern, resource)`：整个资源串按模式匹配（不是子串匹配）。
- `covers(p, q)`：所有能被 `q` 匹配的资源都能被 `p` 匹配，也就是 `q` 描述的范围是 `p` 的子集。
  - `p == "*"` 覆盖一切；
  - `p == q` 互相覆盖；
  - `p` 以 `*` 结尾且 `q` 以 `p` 去掉末尾 `*` 后的字面前缀开头时，`p` 覆盖 `q`（如 `docs/*` 覆盖 `docs/a/*`、`docs/secret`）；
  - 其它情况只有相等才算覆盖。

## §6 快照比对

- 参与比对的主体集合是 `principals` 与 `bindings` 键的并集，任一侧出现过的主体都要比较。
- 每个主体的绑定就是 `bindings[主体]`（缺省为空）。绑定数组按集合理解，顺序变化不算变更；但**绑定内容任何增减都算变更**，即使 `principals` 名单没变，也必须重新计算该主体的有效权限。
- 对每个主体分别计算变更前后的 `effective`（主体被绑定到多个角色时，把每个角色的有效授权并在一起），按下面的规则得出条目：
  - **新增**：after 中存在授权 `g`，before 中不存在任何授权 `h`，使得 `h.action` 与 `g.action` 相同（规范化后）且 `h.resource` 覆盖 `g.resource`。after 中同一 `(action, resource)` 若有多条不同 `path`，每条路径各产生一条新增条目。
  - **移除**：对称地，before 中有、after 中没有覆盖它的授权。
  - **deny 移除**：before 中某主体有效范围内存在 deny `d`，after 中不存在覆盖 `d` 的 deny（动作相同、资源覆盖）时，产生一条 `deny-removed` 条目，记录主体、动作、资源与原来的 path。
- 资源比较一律走 §5 的覆盖关系，不得按字符串相等硬比；动作比较大小写不敏感，并按 §3 展开后的具体动作逐一比较。
- 每条结果的形状为 `Finding(principal, kind, action, resource, path)`，`kind ∈ {add, remove, deny-removed}`。

## §7 报告与级别

`render(diff, before_label, after_label)` 的输出模板固定如下（字段与顺序不得改动）：

```
privpath diff report
before: {before_label}
after: {after_label}
total: {条目总数}

{一行一个条目}
```

每个条目一行，字段顺序固定为：级别、kind、主体、动作、资源、`via`、路径。路径用 ` -> ` 连接：

```
low add alice read docs/x via editor -> reader
high add bob admin * via ops
high deny-removed carol write docs/x via editor
```

kind 的文本就是 `add`、`remove`、`deny-removed`。

级别规则：

- **high**：
  - `add` 且资源模式里含 `*`（包括 `*` 本身和 `docs/*`）；
  - `add` 且动作为 `admin`；
  - 任意 `deny-removed`。
- **low**：其余条目（包括普通的 `remove`）。

报告要求：

- diff 中的每一条 `Finding` 都要出现在报告里；同一主体、同一动作、不同路径的条目必须分别保留，不得按 `(principal, action)` 合并。
- `deny-removed` 条目必须出现，并按上面的规则判为 high。
- 条目按级别（high 在前）→ 主体 → kind → 动作 → 资源 → 路径排序，保证输出稳定。
  级别只影响顺序与卡口，不影响条目是否出现。

## §8 错误与退出码

- 配置错误统一抛出带位置信息的 `ConfigError`。
- CLI 读取两份配置并打印报告：配置错误时退出码为 2；报告里存在 high 条目时退出码为 1；否则为 0。
