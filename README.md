# unpackfence

伙伴上传包的解包层。运营把 tar.gz 或 zip 丢进来，服务按 job id 建
`jobs/<job>/in/`，解包完成后构建步骤直接读这个目录。输入全部来自外部，
解包这一步是信任边界，后面的构建不设防。

## 结构

```
cmd/unpackfence/       命令行入口
internal/archive/      归档读取（tar、tar.gz、zip）
internal/extract/      落盘与逐条目校验
internal/pathguard/    条目名与落点判定
internal/quota/        预算、条目数、压缩比
internal/fault/        拒绝原因
docs/extract-policy.md 行为规范
docs/partner-notes.md  伙伴对接流水（背景材料）
```

## 用法

```
go build ./cmd/unpackfence
./unpackfence -job jobs/abc -pkg upload.tar.gz
```

解包成功时，内容位于 `jobs/abc/in/`。被拒绝时进程以非零码退出，并在
stderr 上给出拒绝原因。

可调上限：

```
-max-bytes    写出的字节数上限（默认 4 GiB，0 表示不限）
-max-entries  条目数上限（默认 200000，0 表示不限）
-max-ratio    压缩比上限（默认 250，0 表示不限）
```

## 包格式

按内容识别 gzip、zip、tar，不依赖扩展名；识别不出来时再看扩展名。
zip 的条目体按压缩流实际内容解出。

## 测试

```
go test ./...
```

现有用例只覆盖正常包：普通文件、空目录、指向包内的符号链接，以及 zip
包与拒绝流程中的错误返回。恶意构造的包不在本仓库的用例范围内。

## 规范

行为口径见 `docs/extract-policy.md`。`docs/partner-notes.md` 是运营记录的
背景流水，不是规范，两者冲突时以 `extract-policy.md` 为准。
