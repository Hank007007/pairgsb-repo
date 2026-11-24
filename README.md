# imapquill

imapquill 是邮件前台与上游 IMAP 服务器之间的协议代理。浏览器端只发送结构化请求
（列出某个文件夹、按条件搜索、批量改标记），由 imapquill 负责把它们变成合法的
IMAP 命令帧、写上线、再把回包归类成前端能理解的结果。

## 目录

```
imapquill/
  proto/     线路原语：astring 引号 / literal、命令帧与长度上限、modified UTF-7
  search/    前端搜索条件与 uid 集
  cmd/       前端操作 -> IMAP 命令帧
  app/       会话、文件夹缓存、请求分发、脚本化上游连接
docs/        规范性文档（行为以这四份为准）
fixtures/    录下来的前端请求
examples/    replay.py，把 fixtures 中的请求跑一遍并打印线路内容
tests/       pytest 用例
```

## 运行

```bash
python -m pytest -q -p no:cacheprovider
python examples/replay.py
python examples/replay.py plain.json
```

只依赖标准库；测试用 pytest。

## 规范

行为以 `docs/` 下的四份文档为准：

* `docs/wire-format.md` — 一条命令怎么拼，astring 何时用 quoted string、何时用
  literal，literal 的字节数与收尾，以及单条命令的 8192 字节上限。
* `docs/search-criteria.md` — 前端搜索键、`CHARSET` 约束、uid set 的形式与规范化。
* `docs/mailbox-names.md` — 文件夹名的 modified UTF-7、`INBOX` 的大小写规则、
  层级分隔符的来源。
* `docs/incidents.md` — 两条事故记录，说明规则从何而来。

`docs/design.md` 描述整体分层与每层的职责，以及一条请求从进入到回包分类的完整
行为要求（不含实现）。

## 快速导览

一条前端请求的路径：

```
request dict
  -> app.handler.dispatch          选择操作，决定结果码
  -> cmd.build.build_*             校验/规范化，拼出命令帧
       -> search.parse.parse_criteria     搜索键 -> SEARCH 参数
       -> search.uidset.parse_uidset      前端 uid 文本 -> uid 集
       -> proto.quote.quote_astring       字符串 -> quoted string / literal
       -> proto.utf7.encode_mailbox       文件夹名 -> modified UTF-7
  -> proto.frame.frame             拼成 "<tag> CMD args CRLF" 并检查 8192
  -> connection.send               写线路，拿回包
  -> app.handler.dispatch          按回包状态归类
```

`app.session` 负责文件夹身份与缓存键：解码后的名字、`INBOX` 归一化、以及按
`LIST` 返回的 delimiter 拼父子路径。
