# keycustody

`keycustody` 是一个阈值密钥托管（threshold key custody）演示程序。主密钥经 Shamir
门限切分后，各分片加密保存；恢复一笔主密钥需要同时满足：

1. 凑够门限数量的同批次分片；
2. 通过多方审批门限与角色策略；
3. 通过完整性摘要校验。

行为规范见 [`docs/custody-design.md`](docs/custody-design.md)。

## 目录结构

```
cmd/keycustody/        命令行入口，串起切分、入库、审批、恢复
internal/shard/        GF(2^8) 上的门限切分与合并
internal/approval/     审批门限与角色策略
internal/store/        AES-GCM 加密的内存分片存储
internal/recover/      恢复编排与敏感数据缓冲
docs/custody-design.md 行为规范
```

## 构建与运行

```sh
go build ./...
go test ./...

# 演示：5 片、门限 3、2 名审批人，打印恢复结果的摘要
go run ./cmd/keycustody

# 打开 debug 日志
go run ./cmd/keycustody -verbose
```

命令行参数：

| 参数 | 默认值 | 含义 |
|---|---|---|
| `-secret` | `correct horse battery staple` | 主密钥内容 |
| `-total` | `5` | 分片总数 N |
| `-threshold` | `3` | 恢复门限 M |
| `-split-id` | `split-a` | 切分批次标识 |
| `-generation` | `1` | 切分代数 |
| `-custody-id` | `tenant-alpha` | 分片所属托管域 |
| `-approvers` | `alice,bob,carol` | 审批人列表 |
| `-required` | `2` | 需要去重后的审批人数 |
| `-role` | `ops-oncall` | 审批人角色 |
| `-verbose` | `false` | 输出 debug 日志 |

## 依赖

仅使用 Go 标准库。
