# 审计链设计说明

`chainlog` 是内部审计日志的落盘层。每条业务动作写一条记录，落成 JSONL，
按条数切段。这份文档写的是**校验必须成立的性质**，以它为准。

## 1. 记录与段

- 一条记录：`{id, ts, actor, action, target, result, meta}` 加上链字段
  `seq`（全局递增，从 1 开始）、`segment`（段号，从 0 开始）、`prevMac`、`mac`。
- 一段 = 一个 `segment-0000.jsonl` 文件，按 `maxEntries` 切。
- `manifest.json` 记当前段号、下一条的 `seq`、最后一条的 `mac`、本段已写条数。
  它只是加速启动的**提示**，不是真相来源 —— 被改过的 manifest 不能让校验通过。

## 2. MAC 的覆盖范围

- 记录 MAC = HMAC-SHA256(key, canonicalize(记录去掉 mac 字段))。
- `canonicalize` 的字段顺序固定，见 `src/canonical.js`。
- **链字段 `seq` / `segment` / `prevMac` 必须被记录 MAC 覆盖。**
  只覆盖业务字段的话，攻击者可以把整条记录挪到别的、改掉序号，MAC 照样对得上。

## 3. 链怎么连

有两条链，缺一不可：

1. **段内链**：一条记录的 `prevMac` == 同段上一条记录的 `mac`。
   段内第一条的 `prevMac` == 上一段最后一条的 `mac`（第一段用 64 个 `0`）。
2. **段间链**：每段有一个段摘要 `segmentDigest`，写在该段最后一条记录上，
   覆盖 `本段最后一条的 mac` + `本段条数` + `段号`。校验时用它确认段没被整体替换、
   也没被截断或追加。

## 4. 校验必须能发现的篡改

下面每一种都必须让 `verifyChain` 返回 `ok: false`，并指出是哪一段、什么原因：

| 篡改方式 | 应发现的原因 |
|---|---|
| 改一条记录的任一业务字段 | `record-mac` |
| 改一条记录的 `seq` 或 `segment` | `record-mac` |
| 改一条记录的 `prevMac` | `record-mac` 或 `chain-link` |
| 删掉段中间的一条 | `chain-link` |
| 删掉某段最后一条 | `segment-digest` |
| 把一段整体替换成另一段（同长度） | `segment-digest` 或 `chain-link` |
| 在某段末尾追加一条 | `segment-digest` |
| 删掉整个中间段文件 | `segment-link`（相邻两段的衔接断了） |
| 删掉段文件后重写 manifest 对齐 | 仍然要发现（manifest 不可信） |

## 5. 报告的形态

```js
verifyChain(dir, key) -> {
  ok: boolean,
  segments: [{ file, entries, ok, reason }],
  broken: { file, seq, reason } | null,   // 第一条出问题的位置
}
```

- `reason` 取值集合见第 4 节表格第一列；没问题的段 `reason: null`。
- 一条链断了以后**继续往下校验**，把后面每段的结论都算出来，
  不要一发现问题就 return —— 审计场景需要知道影响范围。

## 6. 密钥

- 从 `CHAINLOG_KEY` 读，测试里允许显式传入。
- 不落盘、不进日志。
