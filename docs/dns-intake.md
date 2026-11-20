# 上游响应准入规范（dns-intake）

本文件是 `ptrloop` 的行为规范。`internal/wire`、`internal/cache`、`internal/limits`
和 `cmd/dnsscan` 都以本文件为准；任何与本文件冲突的行为都不符合规范，而不是另一种
实现选择。

规范条款编号（§n）在评审、测试与问题登记中直接引用。

---

## §0 范围与信任模型

`ptrloop` 只做一件事：把上游递归返回的**原始响应字节**拆开，判断 answer 段里
哪些记录可以进入本地缓存，并给调用方一个足以拼出回包的 `Message`。

上游不可信：上游可能被投毒、链路中间可能被改写、权威服务器可能被劫持。因此：

- 报文层面：解码必须是严格的。任何与本规范不符的字节都必须导致**整个响应被拒绝**，
  不允许“尽量解析、能读多少读多少”。
- 语义层面：只有同时满足 owner 归属、名字归一、"0x20 回显"和 TTL 规则的记录
  才允许进缓存；其余一拒绝并留下原因。

解析和筛查是两件事：一个响应可以解码成功但一条记录都进不了缓存；也可以因为
报文畸形而整体失败。调用方必须能区分这两种结果。

---

## §1 报文头

报文至少 12 字节，否则整包拒绝。字段按网络序解释：

| 偏移 | 字段 | 规范 |
|---|---|---|
| 0 | ID | 原样保留 |
| 2 | FLAGS | 见下 |
| 4 | QDCOUNT | **只能是 0 或 1**，其它值整包拒绝 |
| 6 | ANCOUNT | 必须等于 answer 段实际记录条数 |
| 8 | NSCOUNT | 必须等于 authority 段实际记录条数 |
| 10 | ARCOUNT | 必须等于 additional 段实际记录条数 |

FLAGS 中与准入相关的位：

- `TC`（0x0200）：置位表示上游明确说明响应被截断。置位的响应**不得**按完整
  响应处理，整包拒绝。
- `QR`、`AA`、`RD`、`RA`、opcode、rcode：原样透传，不在本层做策略判断。

### §1.1 count 与实际记录数一致

三个 section 的 count 是**精确期望值**，不是上限。解析完问题段之后：

- 如果实际能解析出的记录数少于 count，整包拒绝（截断的报文不能伪装成完整报文）。
- 如果按 count 解析完之后 buffer 里还剩字节，整包拒绝（尾部多余字节）。
- 不允许把 count 当“最多读这么多”，也不允许把多出来的记录留在 buffer 里不管。

### §1.2 QDCOUNT

QDCOUNT 大于 1 的响应整包拒绝。QDCOUNT 可以为 0（例如某些错误响应），此时没有
问题段，`Screen` 不做 owner 归属判断（见 §10.4）。

---

## §2 域名解压（`ReadName` / `Label`）

名字由若干标签加一个根终止符构成。每个标签的首字节决定其类型：

| 首字节高两位 | 类型 | 规范 |
|---|---|---|
| `00` | 普通标签 | 低 6 位是标签长度，`0` 表示名字结束 |
| `11` | 压缩指针 | 低 14 位是**报文内**偏移 |
| `01` / `10` | 保留 | **非法**，整包拒绝 |

`Label` 必须把 `01` 和 `10` 开头归为“保留”，不得把它们当普通长度使用。也就是说
首字节 `0x40`–`0xBF` 一律非法。

### §2.1 普通标签

- 单标签长度 1–63 字节；长度 0 表示根/名字结束。
- 标签内容只允许 ASCII 字母、数字和 `-`（`A-Z`、`a-z`、`0-9`、`0x2D`）。
- 标签首字符不得是 `-`。
- 其它字节（含 `_`、空格、高位字节、非 ASCII UTF-8 序列）一律使整包拒绝。
- 标签内容保持原样大小写（大小写只影响比较时的归一，见 §10.2）。

### §2.2 压缩指针

- 指针目标必须**严格小于当前指针所在的偏移**只允许向后指）。指向前方、指向
  自身、或与当前偏移相等的指针一律非法。
- 指针目标必须落在**某个标签的起始字节**上。指向标签中间、指向 RDATA 中间或
  指向任何非标签起点的指针一律非法。
- 一个指针之后的名字继续按普通规则读取，直到遇到终止符或下一个指针。
- 指针链可以有嵌套，但要同时满足 §11 的步数上限。

### §2.3 长度约束

- 每个名字解压之后（即不再有指针的展开形式）总 wire 长度为
  `1 + Σ(1 + len(label))`，必须 ≤ 255 字节。超长整包拒绝。
- 单标签 ≤ 63 字节（由 §2.1 保证）。
- “总长”按解压后计算，而不是按报文里实际占用的字节数计算。一个报文可以用大量
  指针把名字“摊薄”在报文里，但不能绕过 255 字节上限。

### §2.4 终止与展示

- 名字必须以终止符（长度为 0 的标签或根）结束；没有终止符就到 buffer 末尾的，
  视为截断，整包拒绝。
- 解码结果保存为标签序列。展示形式（`String()`）恰好有一个尾点，根名字展示为
  `.`。不得出现 `a.example.com..` 或没有尾点的形式。

### §2.5 循环

指针图必须是有向无环的（由“只向后指”与“目标必须在标签起点”共同保证）。任何
指针环都必须让解码在有限步内终止并返回错误，**不得**挂死、耗尽栈或无限读内存。
典型构造：某个名字的末尾指针指回该名字自己的首字节。

---

## §3 记录解码（`parseRecord`）

每条记录 = 名字 + TYPE(2) + CLASS(2) + TTL(4) + RDLENGTH(2) + RDATA(RDLENGTH)。

- 名字按 §2 解码，且必须落在报文边界内。
- 固定 10 字节字段必须完整存在，否则整包拒绝（不是跳过这条记录）。
- TTL 是 32 位无符号字段，按无符号读取（截断/打标规则见 §8）。
- RDATA 的起点是 `RDLENGTH` 字段之后的那个字节；RDATA 的终点是起点 +
  RDLENGTH。解析器**必须**以这个终点为准。

### §3.1 RDLENGTH 是权威

- RDATA 内容不足 RDLENGTH：整条记录解析失败 → 整包拒绝。
- RDATA 内容多于 RDLENGTH：超出的字节属于后续记录，不得算进本条记录。
- 任何 RDATA 类型的解析都必须在 `[rdStart, rdStart+RDLENGTH)` 内完成；不许
  “解到哪算哪”然后接着读下一条记录。

---

## §4 RDATA 中的域名与压缩

“RDATA 内的地址”一律指**整条报文的起点**（字节 0）。压缩指针的 14 位偏移相对
报文头解释，**不是**相对 RDATA 起点、也不是相对 RDATA 中第一个域名的起点。

这条规则的后果：RDATA 里的域名可以用指针指向该名字在报文中第一次出现的完整编码
位置（通常是 owner 名或问题段），但绝不允许把 RDATA 切片自身当成坐标系。

### §4.1 允许压缩的 RR 类型（白名单）

只有以下类型的 RDATA 允许出现压缩指针：

- `CNAME`（5）
- `NS`（2）
- `MX`（15）——交换器名字允许压缩，preference 字段在前
- `SOA`（6）——MNAME / RNAME 允许压缩

白名单之外的任何类型（包括 `PTR`、`DNAME` 和一切不认识的类型），如果其 RDATA 中
出现压缩指针（首字节 `11` 开头），整包拒绝。不允许“其它类型也顺手解一下压缩名”。
不认识的类型按原始字节保留，但只要出现压缩指针就拒绝。

### §4.2 各类型的长度要求

| 类型 | RDATA 要求 |
|---|---|
| A | 恰好 4 字节 |
| AAAA | 恰好 16 字节 |
| CNAME/NS/PTR/DNAME | 恰好一个名字，且名字编码恰好占满 RDLENGTH |
| MX | preference(2) + 恰好一个名字，占满 RDLENGTH |
| SOA | mname + rname + 5×uint32，占满 RDLENGTH |

“恰好占满 RDLENGTH”意味着 RDATA 里不能有尾随垃圾字节。

---

## §5 OPT 与 EDNS(0)

`OPT`（41）是伪记录：

- owner 必须根（`.`）。
- CLASS 字段被复用为**请求方 UDP payload 大小**。只接受 512–4096（含端点）。
  小于 512 或大于 4096 → 整包拒绝。
- TTL 字段被复用为 extended-rcode/version/flags，本层原样保留，不按 §8 截断。
- RDATA 是 option 列表：`code(2) + len(2) + data(len)`。

### §5.1 option 列表

- 每个 option 的 `code` 和 `len` 都要读全；`len` 超出剩余 RDATA、或 option 列表
  未能恰好占满 RDLENGTH，整包拒绝。
- 未知 code 的 option **必须丢弃**，不得保留在 `RData.Options` 里。
- 已知 code 集合：`NSID`(3)、`COOKIE`(10)、`CLIENT-SUBNET`(8)、`PADDING`(12)、
  `EDE`(15)。只有这些 code 允许出现在结果里。已知 code 的 data 结构不作进一步
  解释，原样保留。
- 已知 code 但 `len` 明显不合法的（例如 COOKIE 长度不是 8–40 字节）整包拒绝。

---

## §6 完整性与尾部

- 解析结束（问题段 + 三个 section 恰好完成）时，读取位置必须**等于**报文长度。
  剩余任何字节 → 整包拒绝。
- `TC=1` → 整包拒绝（§1）。
- 任何“读穿 buffer”的尝试（长度不足）→ 整包拒绝。

---

## §7 失败语义

- 一条记录解析失败**不能**被 `continue` 掉。整个响应失败，`Parse` 返回
  `(nil, error)`。
- 调用方永远不会拿到“部分成功”的 `Message`。要么完整的 `Message`，要么错误。
- 错误必须可判断类别（截断 / 名字非法 / 头非法 / 记录非法 / 预算耗尽），便于
  调用方区分日志与告警。

---

## §8 TTL

TTL 在记录里是无符号 32 位。存入缓存前必须：

1. 以无符号语义读出（不得先转成有符号整数导致 `0xFFFFFFFF` 变成 `-1`）。
2. 上限：大于 7 天（604800 秒）→ 截断到 604800，并打 `Clamped` 标。
3. 下限：值 0 合法。任何“负 TTL”只可能来自把无符号值错误地解释为有符号；
   按无符号读出后这种情形不存在。若上游给的是 `0x80000000` 及以上，按超大值处理：
   截断到 604800 并打标。
4. 截断只影响存缓存的值；`Message` 里保留上游原始 TTL，供日志和回包使用。
5. TTL 打标（`Clamped`）必须传给 `cache.Entry`，供上层决定是否降级或不再续期。

---

## §9 缓存准入（`Screen`）

### §9.1 bailiwick

“在 bailiwick 内”定义为：`owner` 等于 `zone`，或 `owner` 以 `"." + zone` 结尾。
比较必须按**标签边界**进行，而不是裸字符串后缀。

- `www.example.com` 在 `example.com` 的 bailiwick 内。
- `evil-example.com` **不** `example.com` 的 bailiwick 内（它是另一个注册域）。
- `ample.com` 不在 `example.com` 内。
- `example.com.evil.net` 不在 `example.com` 内。

### §9.2 owner 归一

在比较之前，双方名字都要归一：

1. 大小写折叠为小写（DNS 大小写不敏感）。但注意 §9.4 的 0x20 校验使用的是**原始
   回显**，归一后的字符串不用于 0x20 判断。
2. 去掉尾点；比较时双方都不带尾点。`www.example.com` 与 `www.example.com.` 相同。
3. 解释转义写法：`\065` 表示字节 `65`（十进制）= `A`；`\x41` 不是本规范支持的
   形式，按字面处理或拒绝。归一时 `foo\065bar.example.com` 必须等于
   `fooAbar.example.com`（再经大小写折叠后为 `fooabar.example.com`）。
4. 转义只在比较时展开，`Entry.Owner` 可以保留原始展示形式，但判定必须用归一结果。

归一失败（非法转义、非法字节）→ 该记录不可缓存。

### §9.3 CNAME 链

answer 段可以是链：`qname CNAME hop1`、`hop1 CNAME hop2`、`hop2 A ...`。

- 从 `qname` 开始沿着 CNAME 走，每一跳的 owner 必须等于当前链头（按 §9.2 归一）。
- 链上**任何一跳**（包括中间记录和终点记录）的 owner 都必须落在 `qname` 的
  bailiwick 内。任何一跳越界 → **整条链**不进缓存，不只是那一条记录。
- 链的长度也要计入 §10 的记录预算；出现环（`a CNAME b`、`b CNAME a`）→ 整条链
  拒绝。
- 只有“从 qname 可达且每一跳都合规”的记录才可缓存。answer 段里与链无关的记录
  （owner 既不是链头也不在链上）不得缓存。
- 非 CNAME 的终点记录（A/AAAA 等）的 owner 必须等于链头。

### §9.4 0x20 大小写回显

上游响应必须按问题段里 `qname` 的**原始大小写**回显。比较时逐字节比较问题名与
answer 链头 owner 的原始编码（大小写敏感），但只在“可打印字母”位置上要求大小写
一致；数字、连字符、转义不参与。任何位置大小写不一致 → 视为被伪造，整条链拒绝。

### §9.5 结果

`Decision.Cacheable` 只包含通过 §9.1–§9.4 的记录；`Decision.Rejected` 至少包含
被拒 owner 的可读字符串（归一前的展示形式即可），方便 `dnsscan` 打印。

---

## §10 资源预算（`Budget`）

一个 `Budget` 绑定**一次** `Parse` 调用。

- 步数：每跟随一个压缩指针记 1 步，单包累计 ≤ 4096。
- 名字长度：每个解出名字记入一次，长度按 §2.3 的 wire 长度；超 255 直接拒绝，
  且计入预算。
- 记录数：answer + authority + additional 实际解析条数；≤ 512。
- 字节数：报文总字节，用于日志。

### §10.1 先记账后入结果

任何记录/名字/指针在**进入结果切片之前**必须先记账并通过检查。预算超限时，
`Parse` 返回错误，调用方**不会**拿到已超额的记录。

### §10.2 每次解析从零开始

同一 `Budget` 可以被复用（例如同一连接上的连续响应），但每次 `Parse` 开始前必须
由解析器清零；一次解析失败不能把用量带到下一次。`Parse` 成功或失败都必须让下一
次调用从零计数。

### §10.3 超限行为

预算耗尽返回 `limits.ErrExhausted`（或包装它），必须是**错误**而不是部分结果。
解压炸弹报文（大量指针、超长名字、超多记录）必须在一个可预期的时间内返回错误，
不得挂死。

---

## §11 `cmd/dnsscan` 输出契约

对 `testdata/*.bin` 按文件名字典序逐个处理，每个文件输出：

```
== <文件名> (<字节数>)
header: id=<id> flags=[...] opcode=<n> rcode=<n> qd=<n> an=<n> ns=<n> ar=<n>
question: <qname> <TYPE> class=<n>
answer: <条数>
  <owner> <ttl> class=<n> <TYPE> <rdata>
authority: <条数>            （仅当该段非空时输出，下同）
  <owner> <ttl> class=<n> <TYPE> <rdata>
additional: <条数>
  <owner> <ttl> class=<n> <TYPE> <rdata>
cache: owner=<qname> accepted=<n> rejected=<n>
  accept <owner> <TYPE> ttl=<n>[ clamped]
  reject <owner>
budget: steps=<n> names=<n> records=<n> bytes=<n>
```

- OPT 记录的 class 打印为 `udpsize=<n>`，而不是 `class=<n>`。
- `answer:` / `authority:` / `additional:` 三个段标题只在对应段非空时输出；
  问题段为空时 `question:` 行也不输出。

- 解码失败的文件只输出 `== ...`、`error: <err>`、`budget: ...`，然后空行。
- `budget:` 行打印的是**本次解析**的用量；成功和失败都必须让下一次从零开始。
- `accept` 行的 `ttl` 是截断后的值，截断过则带 ` clamped`。
- 输出顺序必须稳定（文件名字典序、记录按报文顺序）。

---

## §12 testdata 清单与期望

所有文件都由 `tools/gen_testdata.py` 生成，可以重新生成。`dnsscan` 按文件名字典序
处理，所以下表也按字典序给出。

| 文件 | 构造 | 期望 |
|---|---|---|
| `bad-ascii-label.bin` | 标签含 `_` 与高位字节 | 名字错误 |
| `case-mismatch.bin` | 问题名 `WwW.Example.COM`，answer owner 全小写 | 解码成功，0 条可缓存（0x20 回显不符） |
| `cname-case-hop.bin` | 链头正确回显，链上第二跳大小写不同 | 解码成功，2 条可缓存（跳内比较大小写不敏感） |
| `cname-chain-in.bin` | `example.com` → `cdn.example.com` → A | 解码成功，2 条可缓存 |
| `cname-chain-out.bin` | 链中间一跳是 `evil-example.com` | 解码成功，整链 0 条可缓存 |
| `count-mismatch.bin` | ANCOUNT = 2，实际 1 条 | 报文错误 |
| `label-too-long.bin` | 单标签 127 字节 | 名字错误 |
| `label-with-dot.bin` | 标签内容为 `bad.`（长度 4） | 名字错误 |
| `name-too-long.bin` | 解压后名字 > 255 字节 | 名字错误 |
| `normal-a.bin` | 正常 `www.example.com A` 响应 | 解码成功，1 条 answer 可缓存 |
| `nxdomain.bin` | `nope.example.com` 的 NXDOMAIN | 解码成功，0 条 answer，0 条可缓存 |
| `opt-bad-udpsize.bin` | OPT class = 256 | 报文错误 |
| `opt-unknown-option.bin` | OPT 含未知 code 65001 与已知 code 3 | 解码成功，只剩 code 3 |
| `partial-records.bin` | 声明 3 条 answer，中间一条 owner 指针越界 | 报文错误，不得返回前两条 |
| `ptr-bomb.bin` | 100 条记录的名字串成指针链，共需 4950 步 | 预算耗尽错误，有限时间内 |
| `ptr-forward.bin` | 指针指向前方偏移 | 名字错误 |
| `ptr-mid-label.bin` | 指针指向某个标签的中间字节 | 名字错误 |
| `ptr-self-loop.bin` | `a` 标签的末尾指针指回该名字自身 | 名字错误或预算耗尽，不得挂死 |
| `qdcount-2.bin` | QDCOUNT = 2 | 报文错误 |
| `rdata-bad-rdlength.bin` | CNAME 的 RDLENGTH 小于名字长度 | 记录错误，不得读穿到下一条 |
| `rdata-compressed-unknown.bin` | `TYPE65280` 的 RDATA 以压缩指针开头 | 记录错误 |
| `rdata-name-pointer.bin` | CNAME 的 RDATA 是 `c0 0c`，指向问题名 `cdn.example.com` | 解码成功，CNAME 目标为 `cdn.example.com`；owner `www.example.com` 的 CNAME 不在链上被拒，`cdn.example.com` 的 A 记录可缓存 |
| `reserved-label.bin` | 标签首字节 `0x40` | 名字错误 |
| `suffix-confusion.bin` | 问题名 `example.com`，answer owner 是 `evil-example.com` | 解码成功，0 条可缓存 |
| `tc-set.bin` | TC = 1 | 报文错误 |
| `trailing-bytes.bin` | 正常响应尾部多 3 字节 | 报文错误 |
| `ttl-huge.bin` | TTL = 0xFFFFFFFF | 解码成功，缓存 ttl = 604800 且打标 |
| `ttl-wrap.bin` | TTL = 0x80000000 | 解码成功，缓存 ttl = 604800 且打标 |
| `ttl-zero.bin` | TTL = 0 | 解码成功，缓存 ttl = 0，可缓存 |
| `two-responses.bin` | 两份正常响应拼接 | 尾部多余字节 → 报文错误 |

> 说明：`two-responses.bin` 不是一个合法 DNS 报文。它用于验证“尾部多余字节”
> 这条规则，同时也能让人手工验证 §10.2：`dnsscan` 对每个文件调用一次 `Parse`，
> `budget:` 行必须只反映那一次解析的用量。双包共用一个 `Budget` 的场景由
> `internal/limits` 的单元测试覆盖。

---

## §13 不在本层负责的事

- 递归解析、上游选择、超时重试。
- DNSSEC 验证。
- 过滤策略（黑名单/白名单）。
- 回包拼装与截断到客户端 UDP 大小。

本层只负责“按上面的规则把字节解对、把该收的记录收进来”。
