# glassbreak — break-glass elevation service

`glassbreak` 是内部系统的紧急提权服务。平时的运维账号只有只读权限；发生需要更高权限的故障时，
值班人员提交一份提权申请，写明要追加什么 scope、原因、需要多久，由同组审批人投票，达到票数门槛后
签发一张短期的、自动到期的凭据，全流程留审计。

本仓库是一份可运行的实现，包含策略数据、服务代码、命令行演示与基础测试。

## 目录结构

```
glassbreak/            服务实现
  models.py            ElevationRequest / Credential / RequestStatus
  policy.py            grantable.yaml 与 policy/groups.yaml 的加载与查询
  elevate.py           scope 规划
  approval.py          审批投票
  credential.py        凭据签发与台账
  audit.py             审计记录
  service.py           对外门面 GlassbreakService
  cli.py               python -m glassbreak 的演示流程
  _yaml.py             仅覆盖策略文件所用 YAML 子集的最小读取器
tests/                 pytest 用例
grantable.yaml         可提权 scope 集合与 max_ttl
policy/groups.yaml     组、成员、基础 scope、票数门槛、审批窗口
docs/elevation-rules.md  规则说明（权威行为定义）
```

## 运行

```bash
python -m glassbreak            # 走一遍提权演示，打印审计记录
python -m pytest -q -p no:cacheprovider
```

运行时只依赖 Python 标准库；`pytest` 仅用于测试。

## 快速上手

```python
from datetime import timedelta

from glassbreak import GlassbreakService, load_policy

service = GlassbreakService(load_policy())
request = service.create_request(
    subject="alice",
    requested_scopes=["db:write"],
    reason="replica lag during incident INC-4471",
    duration=timedelta(minutes=45),
)
service.approve(request.request_id, "bob")
service.approve(request.request_id, "carol")
credential = service.issue_credential(request.request_id)
print(credential.scopes, credential.expires_at)
```

`GlassbreakService` 接受可选的 `clock` 参数（返回 aware `datetime` 的无参函数），
便于在测试或演练中推进时间。

## 数据

- `grantable.yaml` — 可追加的 scope 目录，以及凭据时长的上限 `max_ttl_minutes`。
- `policy/groups.yaml` — 每个组的成员、基础 scope、审批所需票数，以及全局的 `approval_window_minutes`。

这两份文件是配置数据，服务只读取，不在运行期改写。

## 规则

`docs/elevation-rules.md` 是行为规范的唯一来源：scope 如何合并、谁能投票、票怎么算、
审批窗口、凭据 TTL 与起算点、凭据唯一性，都以那份文档为准。
