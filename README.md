# netacl

`netacl` 是私有云控制面里的安全组求值器。编排器在下发规则之前、drift 检测器在巡检时都会调用它，回答同一个问题：给定的源（CIDR、prefix list 或另一个安全组）能不能连到目标安全组上的某个协议和端口。

语义以 `docs/eval-semantics.md` 为准。`docs/incident-reports.md` 收录了两次放错流量的复盘，可以作为回归场景的来源。

## 构建与测试

```sh
go build ./...
go test ./...
go run ./cmd/netacl-demo
```

只依赖 Go 标准库。模块声明在 `go.mod`（`go 1.22` 及以上）。

`cmd/netacl-demo` 会把复盘里的几组场景跑一遍并打印原始判定结果，用于从终端直接观察求值器行为；它不做断言，也不参与测试。

## 目录

| 路径 | 内容 |
|---|---|
| `internal/acl/rule.go` | 规则、规则编号、规则簿、安全组 |
| `internal/acl/match.go` | 协议与端口判定 |
| `internal/acl/group.go` | 组成员目录与组引用解析 |
| `internal/acl/prefix.go` | prefix list 与版本化目录 |
| `internal/acl/eval.go` | 求值入口、连接表、回程判断 |
| `internal/acl/snapshot.go` | 一次求值所用的固定快照 |
| `docs/` | 规范与复盘 |

## 调用方式

```go
reg := acl.NewRegistry()
reg.SetGroup(acl.SecurityGroup{
    ID: "sg-app",
    Ingress: acl.NewRuleBook(
        acl.Rule{Number: 100, Action: acl.ActionAllow, Protocol: acl.ProtocolTCP,
            FromPort: 443, ToPort: 443, Source: acl.CIDRSource("10.0.0.0/8")},
    ),
})

ev := acl.NewEvaluator(reg)
decision, err := ev.Evaluate(acl.Query{
    GroupID:   "sg-app",
    Direction: acl.Ingress,
    Flow: acl.Flow{
        Src: netip.MustParseAddr("10.1.2.3"), Dst: netip.MustParseAddr("10.9.9.9"),
        SrcPort: 51000, DstPort: 443, Protocol: acl.ProtocolTCP,
    },
})
```

`acl.Registry` 是策略输入的持有者：安全组、组成员关系、prefix list。每次 `Evaluate` 自行从注册表取一份快照，调用方不需要手工管理版本。

已建立的连接通过 `ev.Connections().Establish(flow)` 登记，回程流量据此免于重新查规则。
