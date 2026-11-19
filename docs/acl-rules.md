# ACL 规范与设计说明

本文描述 brokeracl 插件**应当**具备的对外行为。它是规范性文档：实现与本文冲突时以本文为准。
文中不规定内部数据结构与算法，只规定可观察的结论。

约定：`MUST` 表示强制，`MUST NOT` 表示禁止，`SHOULD` 表示除非有充分理由否则应遵守。

---

## §1 作用域与默认策略

1.1 插件对三类问题给出是/否结论：

| 动作 | 输入 | 含义 |
|---|---|---|
| `publish` | 具体 topic 名 | 该客户端能否向此 topic 发布 |
| `subscribe` | topic filter | 该客户端能否订阅此 filter |
| 接收判定 | 具体 topic 名 | 该客户端此刻能否收到发往此 topic 的消息 |

1.2 接收判定（retained 投递、在线投递）按 `subscribe` 类规则、以**具体 topic** 为对象求值。

1.3 默认拒绝：只有当某个 principal 的一条 `allow` 规则命中、且没有任何 `deny` 规则命中时，结论才是允许。
无规则、principal 不存在、动作不匹配、subject 不匹配，均为拒绝。

1.4 任何解析失败、I/O 失败或异常都 `MUST` 收敛为拒绝（fail closed），不得放行。

---

## §2 Principal 与身份映射

2.1 客户端通过认证后带有三种可选身份属性：证书 CN、用户名、clientId。

2.2 规则文件以带前缀的 key 组织 principal，映射关系固定为：

| 身份属性 | principal key |
|---|---|
| 证书 CN | `cn:<CN>` |
| 用户名 | `user:<username>` |
| clientId | `client:<clientId>` |

2.3 一次判定 `MUST` 考虑该客户端映射出的**全部** principal，取并集语义：

- 任一 principal 给出 `deny`，整体结论为拒绝；
- 否则任一 principal 给出 `allow`，整体结论为允许；
- 都没有命中则拒绝。

2.4 principal 缺失的属性不参与匹配。未认证连接只有 clientId 时只匹配 `client:` 条目。

2.5 规则文件中的 principal 名区分大小写，`cn:` / `user:` / `client:` 前缀本身为固定小写。

---

## §3 Topic filter 语法

3.1 topic 与 filter 以 `/` 分层。空层是合法且有意义的层（`a//b` 是三层），空层不等于通配符。
比较始终区分大小写。

3.2 `+` 匹配**恰好一层**，且必须独占一层。`a/+/c` 合法；`a+/c`、`a/b+` 为非法。

3.3 `#` 匹配**零层或多层**，且只能出现在 filter 的最后一层。`a/#` 合法；`a/#/b`、`a/b#` 为非法。

3.4 `a/#` 覆盖 `a` 本身以及 `a` 下任意层级的 topic；`#` 单独出现时覆盖全部 topic。

3.5 规则文件中的 `topic` 字段是 filter，`MUST NOT` 写成 `$share/...` 形式；共享订阅是客户端订阅语法，不是规则对象（见 §6）。

3.6 规则文件结构：

```json
{
  "principals": {
    "user:alice": {
      "rules": [
        { "action": "publish",   "topic": "sensors/+/state", "effect": "allow" },
        { "action": "subscribe", "topic": "sensors/#",        "effect": "deny" }
      ]
    }
  }
}
```

字段缺省或取值非法（`action` 不是 `publish`/`subscribe`，`effect` 不是 `allow`/`deny`，`topic` 不满足 §3.1–3.3）时，整个规则文件加载失败，且 `MUST NOT` 部分生效。

3.7 规则目录下所有 `*.json` 按文件名字典序合并；同一 principal 的规则跨文件累加。

---

## §4 规则 filter 对 subject 的覆盖关系

4.1 覆盖关系的判定单位是**层**：一段规则 filter 只能对 subject 的对应一段生效。层级边界是 `/`；
`sensors` 与 `sensors2`、`a/b` 与 `a/b/c`、`a/b` 与 `a/bc` 之间不存在覆盖关系。

4.2 对 `publish` 判定，subject 是具体 topic：

- 规则 filter 的层数与 topic 相同，且逐层相等，则覆盖；
- 规则 filter 的某层为 `+`，覆盖 topic 的对应层任意一个值；
- 规则 filter 的最后一层为 `#`，覆盖 topic 中从该位置起的零层或多层；
- 其余情况不覆盖。

由此：

| 规则 filter | 覆盖 | 不覆盖 |
|---|---|---|
| `sensors/#` | `sensors`、`sensors/a`、`sensors/a/b` | `sensor`、`sensors2`、`a/sensors` |
| `sensors/+` | `sensors/a`、`sensors/b` | `sensors`、`sensors/a/b` |
| `a/b` | `a/b` | `a/b/c`、`a`、`a/bc` |
| `+/state` | `a/state` | `a/b/state`、`state` |

4.3 对 `subscribe` 判定，subject 是客户端请求的 filter（可能自身含通配符），规则 filter 必须覆盖该请求所可能选中的**每一个** topic：

- 请求 filter 的每一层都必须被规则 filter 的对应层覆盖；
- 规则 filter 的 `+` 覆盖请求中任意单层（包括具体值、`+`；不含 `#`）；
- 规则 filter 的 `#` 覆盖请求中从该位置起的任意剩余层（包括 `#` 以及无剩余层）；
- 请求 filter 中出现 `#` 时，规则 filter 对应位置也必须是 `#`。

例：规则 `sensors/#` 覆盖请求 `sensors/+`、`sensors/a/#`、`sensors`；规则 `sensors/+` 只覆盖 `sensors/+`、`sensors/a`，不覆盖 `sensors/#`。

4.4 覆盖关系与规则书写顺序无关；它只回答“这条规则是否适用于该 subject”。

---

## §5 冲突消解：deny 优先

5.1 同一 principal、同一动作、同一 subject 上若有多条规则同时覆盖，`deny` 一律优先于 `allow`。

5.2 该优先关系与规则在文件中的先后、跨文件合并的先后**完全无关**。把 `deny` 写在 `allow` 之后，结果必须与写在之前相同。

5.3 一个请求的结论为允许，当且仅当：至少一条 `allow` 覆盖它，且不存在任何覆盖它的 `deny`（包括其它 principal 上的 `deny`，见 §2.3）。

5.4 因此规则集合的语义是“白名单减去黑名单”，而不是“第一条命中者胜出”。

---

## §6 共享订阅

6.1 客户端订阅语法允许 `$share/<group>/<filter>`：`<group>` 是共享组名（非空、不含通配符），`<filter>` 是 §3 意义下的普通 topic filter。

6.2 **鉴权对象是剥掉 `$share/<group>/` 前缀之后的 `<filter>`**，而不是客户端发送的整串。组名不是 topic 的一部分。

6.3 因此：

- 若客户端对 `<filter>` 有订阅权限，则 `$share/<group>/<filter>` 必须允许；
- 若客户端对 `<filter>` 无订阅权限（含默认拒绝），则 `$share/<group>/<filter>` 必须被拒绝；
- 同一 `<filter>` 在不同组下的鉴权结论相同；
- 组与组之间只在投递调度策略上不同，不影响鉴权；本文不规定调度算法。

6.4 形如 `$share`、`$share/g`、`$share//x`、`$share/g/`、`$share/g/$share/h/x` 的订阅请求为非法，直接拒绝。

6.5 普通订阅（不以 `$share/` 开头）按 §3、§4 原样鉴权。

---

## §7 Retained 消息

7.1 订阅成功时，broker 会把与订阅 filter 匹配的 retained 消息投递给该客户端。

7.2 retained 消息的投递 `MUST` 在**投递那一刻**按当前生效的 ACL 重新判定，判定对象是该 retained 消息的**具体 topic**、动作按 §1.2。

7.3 订阅时缓存的授权结论只说明“订阅请求本身被允许”，`MUST NOT` 被当作后续每次投递的通行证。

7.4 因此规则被 reload 收紧后，即使客户端仍持有旧的、当时合法的订阅，也不得再收到已无权限的 retained 消息。reload 放宽时，旧订阅在下次投递时应能收到新放开的 retained 消息。

7.5 被拒绝的 retained 消息静默跳过，不影响同一批里其它消息的投递。

---

## §8 Will 消息

8.1 客户端连接时可登记一条遗嘱消息（topic、payload、retain、qos）。

8.2 遗嘱在非正常断开时由 broker 代为发布。它代表的是**该客户端**的发布行为。

8.3 发布遗嘱前 `MUST` 以该客户端的认证身份、按当前 ACL 执行 `publish` 判定，subject 为遗嘱的具体 topic：

- 有权限：按普通消息发布并投递；
- 无权限：遗嘱 `MUST NOT` 被发布，也 `MUST NOT` 被写入 retained 存储，且不得扇出给任何订阅者。

8.4 遗嘱的鉴权不因“来自 broker 内部通道”而豁免；内部通道不是绕过 ACL 的理由。

8.5 正常断开（clean disconnect）不触发遗嘱。

---

## §9 规则热加载

9.1 收到 `SIGHUP` 时重新读取规则目录并整体替换规则集合。

9.2 替换是原子的：新规则要么全部生效，要么完全不生效。加载失败时保留旧规则，并记录错误。

9.3 新规则对替换之后发生的所有判定立即生效，包括已存在订阅的在线投递（§1.2）与 retained 投递（§7.2）。

9.4 版本号单调递增，便于日志与排查。

---

## §10 可观测性

10.1 每次拒绝 `SHOULD` 能说明原因类别：`default-deny`、`matched-deny`、`invalid-filter` 等。

10.2 决策结果 `SHOULD` 携带命中的 principal 与规则，便于审计；不得输出 payload 内容。
