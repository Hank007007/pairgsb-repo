# verbgate 设计说明

verbgate 是自建控制面的授权 webhook。API server 在处理每个请求前，把请求打包成 SubjectAccessReview 发到 verbgate；verbgate 返回 allow、deny 或 no-opinion。本文说明系统由哪些部分组成、数据如何流动，以及实现必须维持的行为契约。具体语义以 `docs/authz-semantics.md` 为准。

## 1. 目标与非目标

目标：

- 用 Role / ClusterRole + RoleBinding / ClusterRoleBinding 表达授权。
- 支持 user 与 group 主体，group 可由 `groups.yaml` 嵌套定义。
- 对每个请求给出确定、可审计的 allow / deny / no-opinion。
- 通过判定缓存降低高频请求的求值开销，同时不返回过期或跨主体的结论。

非目标：

- 不实现认证。进入 webhook 的请求已经过 API server 认证。
- 不提供写入角色的接口。角色与绑定来自控制面下发的快照。
- 不实现 Kubernetes 全部资源语义，只实现 `docs/authz-semantics.md` 描述的部分。

## 2. 组件

```
API server
   │  POST /authorize  (SubjectAccessReview)
   ▼
webhook handler ──► authorizer ──► cache
                        │  │
                        │  └──► groups.yaml (subject.Source)
                        └─────► snapshot (store.Store)
                                   ├── Roles
                                   ├── ClusterRoles
                                   ├── RoleBindings
                                   └── ClusterRoleBindings
```

- **review**：请求与响应的数据结构。字段名与 JSON tag 是对外契约，不随实现调整。
- **subject**：从 `groups.yaml` 读取组成员关系，并负责把主体展开成最终组集合。
- **store**：持有角色与绑定的只读快照；快照带版本号，重新加载时整体替换。
- **authz**：规则匹配（单条 rule 与请求）与决策汇总（同一层内多条 rule 的优先级）。
- **cache**：保存最近判定，容量有限，按插入顺序淘汰。
- **webhook**：HTTP 入口，解码请求、调用授权器、编码响应。

## 3. 请求生命周期

1. handler 解码 SubjectAccessReview。缺少 user 或 resource 的请求直接返回 no-opinion。
2. 确定有效身份：有 impersonation 时使用被模拟的 user / groups，否则使用请求自身的 user / groups。
3. 通过 `subject.Source` 展开组集合，并补充内置组。
4. 以"完整请求 + 有效身份 + 当前快照版本"查询缓存；命中则直接返回结论。
5. 未命中时，收集适用于该请求的绑定：请求 namespace 内的 RoleBinding，加上全部 ClusterRoleBinding；集群级请求只用 ClusterRoleBinding。
6. 对每个绑定的主体求交，再对其引用的角色逐条规则匹配，汇总为单一结论。
7. 将结论写入缓存并映射为响应。

## 4. 行为契约

以下契约是验收依据，详细条款见 `docs/authz-semantics.md`。

- **资源身份**：`resource` 与 `subresource` 是两个维度，按 `资源/子资源` 整段比较；子资源必须单独授权（§2）。
- **通配符**：`*` 覆盖所在维度的全部取值，`resources: ["*"]` 必须覆盖子资源（§3）。
- **resourceNames**：非空时只对具名对象生效，名字为空的集合级请求不匹配（§4）。
- **优先级**：同一层内 deny 优先于 allow，结论与遍历顺序无关（§5）。
- **组展开**：递归到不动点，处理环；每个已认证主体都带 `system:authenticated`（§1）。
- **绑定作用域**：RoleBinding 限于自身 namespace，ClusterRoleBinding 全局（§6）。
- **缓存**：键覆盖完整请求、有效身份与快照版本；快照换版或主体不同都不得复用（§7）。
- **响应**：allow / deny / no-opinion 三态映射明确，no-opinion 不等于放行（§8）。

## 5. 数据文件

### 5.1 groups.yaml

```yaml
users:
  alice:
    - platform
groups:
  platform:
    - sre
  sre:
    - db-oncall
  db-oncall: []
```

`users` 段给出用户直接所属的组；`groups` 段给出组之间的包含关系。文件在启动时解析一次，格式错误视为启动失败。

### 5.2 快照

快照为 JSON，包含版本号、Roles、ClusterRoles、RoleBindings、ClusterRoleBindings。store 只暴露只读查询，重新加载时以新快照整体替换旧快照，版本号随之变化。缓存必须感知版本变化。

## 6. 失败与边界

- `groups.yaml` 中的环不得导致展开死循环。
- 同一个组在展开结果中只出现一次。
- 缓存容量满时淘汰最旧条目；被淘汰的结论不得再返回。
- 角色引用不存在时，该绑定视为不产生任何效果。
- 所有判定结果只取决于请求、身份、组关系与当前快照，不依赖历史请求。
