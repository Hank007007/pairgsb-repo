# 响应构造规范

本文件规定 rangecarve 对外发出的状态码、响应头与 body 形态。

## §1 状态码

§1.1 `200 OK`：返回整个对象。

§1.2 `206 Partial Content`：返回一个或多个分段。单段与多段分别见 §2 与 §3。

§1.3 `304 Not Modified`：条件请求命中，见 `conditional.md` §3；无 body。

§1.4 `412 Precondition Failed`：条件请求不满足，见 `conditional.md` §4；无 body。

§1.5 `416 Range Not Satisfiable`：`Range` 不可满足，见 `range.md` §2.3；无 body。

§1.6 `400 Bad Request`：`Range` 语法非法，见 `range.md` §2.1；无 body。

§1.7 `502 Bad Gateway`：上游不可用或返回了无法转发的响应。

## §2 单段响应

§2.1 `Content-Range` 的格式是 `bytes <first>-<last>/<size>`，`first` 与 `last`
都是闭区间的端点，两端都包含在 body 里。1000 字节对象上的 `bytes=0-0`
写出的 `Content-Range` 是 `bytes 0-0/1000`，body 恰好 1 字节。

§2.2 `Content-Length` 等于 `last-first+1`。

§2.3 未知长度的对用 `*` 作总长度，形如 `bytes 0-0/*`。

## §3 多段响应

§3.1 `Content-Type` 为 `multipart/byteranges; boundary=<boundary>`，
`<boundary>` 每次响应都应当是新的，且不出现在对象内容里。

§3.2 每个 part 以 `--<boundary>\r\n` 开头，随后是 part 头，至少包含
`Content-Range` 与 `Content-Type`，其中 `Content-Range` 用 §2.1 的闭区间写法。
part 头之后是一个空行，然后是该段的原始字节，随后是 `\r\n`。

§3.3 整段 body 以 `--<boundary>--\r\n` 结束。

§3.4 响应整体的 `Content-Length` 等于所有 part（含分隔符、part 头、空行、
段间与结尾的 `\r\n`）拼起来的总字节数。它不等于任何单段的长度。

§3.5 part 的个数与顺序跟请求中的段一一对应。

## §4 下载头

§4.1 只有对象作为附件下载时才发 `Content-Disposition`；`inline` 与 `attachment`
两种处置类型都可能出现。头中必须出现 `filename` 或 `filename*` 之一。

§4.2 文件名只能出现在 `Content-Disposition` 的引号字符串里，或者按 RFC 5987
编码后出现在 `filename*` 中。§4.3 文件名里的 `"`、`\`、控制字符（包括 CR 与
LF）都不得原样出现在引号字符串里：`"` 与 `\` 用反斜杠转义，控制字符不得
出现在头中。

§4.4 非 ASCII 文件名应当同时给出回退的 `filename`（只含可打印 ASCII，非 ASCII
字符替换为 `_`）和 `filename*`（`UTF-8''` 前缀加百分号编码），两者写在同一个
`Content-Disposition` 里。

§4.5 一个响应里只有一条 `Content-Disposition`。文件名无论包含什么，都不能
让头列表多出条目或改写后面的 `Content-Type`、`Content-Length` 等头。

## §5 通用头

§5.1 任何带 body 的响应都要有可计算的 `Content-Length`。

§5.2 `Accept-Ranges: bytes` 出现在所有可能返回分段的响应里，包括 `200`、
`206` 与 `416`。

§5.3 `ETag` 与 `Last-Modified` 在 `200` 与 `206` 上回传；`304` 至少回传
`ETag`。

§5.4 `Cache-Control` 与 `Content-Type` 取自上游元数据，缺省时
`Content-Type` 用 `application/octet-stream`。
