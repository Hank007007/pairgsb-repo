# 条件请求规范

本文件规定 rangecarve 如何评估 `If-Match`、`If-None-Match`、`If-Modified-Since`、
`If-Unmodified-Since`、`If-Range` 五个请求头，以及它们与 `Range` 的关系。

## §1 校验值

§1.1 对象有两个校验值：实体标签（`ETag`）和最后修改时间（`Last-Modified`）。
ETag 由上游给出，形如 `"v3"`（强校验值）或 `W/"v3"`（弱校验值）。最后修改时间
是带时区的时间戳，精度至少到秒，比较时按 §1.3 处理。

§1.2 当 ETag 与 Last-Modified 都能参与同一个判断时，ETag 优先。具体地：请求里一旦
出现了 `If-Match` 或 `If-None-Match`，`If-Modified-Since` 与 `If-Unmodified-Since`
就不再参与判定，无论 ETag 条件本身是否匹配。只有在没有任何 ETag 条件头的请求上，
时间头才生效。

§1.3 HTTP-date 本身只精确到秒，因此请求头解析出来的时间是秒级的；对象自己的最后
修改时间可能带秒以下的部分。两者比较时按完整时刻比较，不得把对象的最后修改时间
截断到秒之后再比。

## §2 If-Range

§2.1 `If-Range` 只认强校验值。它的值与对象当前 ETag 逐字节相等（包含两侧引号）
时才匹配。带 `W/` 前缀的值、按 HTTP-date 解释的值、以及任何与当前 ETag 不相等
的值都算不匹配。

§2.2 `If-Range` 不匹配时，忽略请求中的 `Range`，改为返回 `200` 全量；匹配时
`Range` 正常生效。`If-Range` 没有伴随 `Range` 时不起作用。

§2.3 `If-Range` 不产生 `304` 也不产生 `412`。它只决定 `Range` 用不用。

## §3 If-None-Match

§3.1 `If-None-Match` 的值可以是逗号分隔的多个实体标签，也可以是 `*`。

§3.2 匹配的判定：请求值 `*` 时只要对象存在即匹配；否则当请求中任一实体标签
与对象当前 ETag 相等（弱比较，两侧 `W/` 前缀都忽略）时匹配。

§3.3 `GET` 与 `HEAD` 上匹配时返回 `304 Not Modified`，响应不带 body，并带上
`ETag`。§3.4 其它方法上匹配时返回 `412 Precondition Failed`。

§3.5 不匹配时继续按其它头决定后续行为。

## §4 If-Match

§4.1 `If-Match` 的值可以是逗号分隔的多个实体标签，也可以是 `*`。

§4.2 匹配的判定按强比较：两侧都不能带 `W/`，且标签逐字节相等。请求值 `*`
时只要对象存在即匹配。

§4.3 不匹配时返回 `412 Precondition Failed`，响应不带对象 body。

§4.4 匹配时继续按其它头决定后续行为。

## §5 时间头

§5.1 `If-Modified-Since` 只对 `GET`、`HEAD` 生效。对象的最后修改时间不晚于该
时间时返回 `304`，否则继续。

§5.2 `If-Unmodified-Since` 在对象最后修改时间晚于该时间时返回 `412`，否则继续。

§5.3 无法解析成 HTTP-date 的时间头一律忽略。

## §6 判定顺序

§6.1 依次检查：`If-Match`（不匹配 `412`）→ `If-None-Match`（`GET`/`HEAD` 匹配
`304`，其它方法匹配 `412`）→ 时间头（`If-Modified-Since` / `If-Unmodified-Since`，
仅当请求里没有任何 ETag 条件头时才参与）→ `If-Range`（不匹配则丢弃 `Range`）。

§6.2 任何一步给出终态（`304` 或 `412`）后不再检查后续条件。

§6.3 全部通过后，`If-Range` 不匹配则忽略 `Range` 返回全量；否则按 `range.md`
处理 `Range`。
