# verbgate 授权语义

本文是 verbgate 的行为规范。实现必须与本文一致；本文只描述"应该怎样"，不描述代码结构。

术语：

- **主体（subject）**：发起请求的用户，连同其所属的组集合。
- **请求（request）**：一次 SubjectAccessReview 所描述的 verb / apiGroup / resource / subresource / namespace / name。
- **规则（rule）**：Role 或 ClusterRole 中的一条 `rules` 条目，含 `verbs`、`apiGroups`、`resources`、`resourceNames` 与效果（allow / deny）。
- **绑定（binding）**：RoleBinding 或 ClusterRoleBinding，把角色授予若干 user 或 group。

## §1 主体与组展开

1.1 主体的组集合由三部分组成：请求里直接携带的组、`groups.yaml` 中由这些组可达的全部组、以及服务端补充的内置组。

1.2 `groups.yaml` 的 `groups` 段定义组与子组的包含关系：若组 A 的成员列表包含组 B，则 B 的每个成员也是 A 的成员。包含关系可以任意嵌套，展开须递归到不动点，不能只展开一层。展开过程中遇到环（A 包含 B、B 包含 A）时不得死循环，每个组只出现一次。

1.3 每个通过认证的主体都属于内置组 `system:authenticated`。无论请求里是否显式携带该组，也无论用户是否出现在 `groups.yaml` 中，展开后的组集合都必须包含它。该组可以像普通组一样被 RoleBinding / ClusterRoleBinding 引用。

1.4 请求携带 `impersonation` 时，被模拟的身份（user 与 groups）取代原始请求者参与全部判定；`groups.yaml` 的展开与内置组补充作用在模拟身份上。

1.5 组集合是集合语义：重复的组只保留一份，顺序不影响判定结果。

## §2 资源标识与子资源

2.1 资源身份由 `resource` 与 `subresource` 两段组成：无子资源时资源标识就是 `pods`，有子资源时是 `pods/exec`。

2.2 规则 `resources` 中的条目与请求的资源标识按整段比较，不做前缀比较。`resources: ["pods"]` 不匹配 `pods/exec`，反之亦然。

2.3 子资源是独立授权对象：能操作 `pods` 不代表能操作 `pods/exec` 或 `pods/log`，必须为目标子资源单独授予。授予 `pods/exec` 也不会顺带授予 `pods` 本身。

2.4 请求未携带 `subresource` 时，规则条目中的 `pods` 与请求的 `pods` 相同即命中；规则条目写成 `pods/exec` 的规则不命中无子资源的请求。

## §3 通配符

3.1 `verbs`、`apiGroups`、`resources` 的条目取值为 `*` 时代表该维度上的全部取值。

3.2 `resources: ["*"]` 覆盖所有资源，并且覆盖所有子资源——包括 `pods/exec` 这类带子资源的请求。`*` 不是"资源名前缀"，不得因为资源标识里带 `/` 而失效。

3.3 `verbs: ["*"]` 覆盖任意动词；`apiGroups: ["*"]` 覆盖任意 API group（包括 core 组的空字符串）。

3.4 通配符只作用于它所在的维度，不隐式放宽其他维度。例如 `resources: ["*"]` 仍然受 `verbs`、`apiGroups`、`resourceNames` 约束。

## §4 resourceNames

4.1 `resourceNames` 为空（缺省）时，规则对任意名字的对象生效，也对该资源的 list / watch 等不带名字的请求生效。

4.2 `resourceNames` 非空时，规则只对名字明确出现在列表中的对象生效。请求的 `name` 为空（list、watch、create 等集合级操作）时，写了 `resourceNames` 的规则一律不匹配。

4.3 名字按完整字符串比较，不做前缀、通配或大小写归一。`resourceNames: ["db-prod"]` 不匹配 `db-prod-2`。

4.4 `resourceNames: ["*"]` 与缺省不同：它表示"所有具体名字"，仍要求请求带有具体名字；集合级请求不匹配。

4.5 `resourceNames` 只约束"对象名字"这一维度，不改变子资源语义：`resourceNames: ["db-prod"]` 对 `pods/exec` 上的请求同样要求子资源被单独授权。

## §5 deny 与 allow 的优先级

5.1 每条规则带有 allow 或 deny 效果。缺省效果为 allow。

5.2 判定在同一层内汇总：只要有一条 deny 规则命中，结论就是 deny，无论是否有 allow 规则同样命中；allow 不能覆盖 deny。

5.3 只有在没有任何 deny 命中、且至少有一条 allow 命中时，结论才是 allow。

5.4 没有任何规则命中时，结论为 no-opinion（等同于不干预，由上层决定）。no-opinion 不是 allow。

5.5 优先级不因规则或绑定的遍历顺序而改变。先遇到 allow 后遇到 deny，与先遇到 deny 后遇到 allow，结论必须相同。

## §6 绑定作用域与求值

6.1 RoleBinding 只在其所在的 namespace 内生效。只有当请求的 namespace 与绑定所在 namespace 相同时，绑定所授予的规则才参与判定。

6.2 ClusterRoleBinding 在集群范围内生效，对任意 namespace 的请求都参与判定。

6.3 RoleBinding 的 `roleRef.kind` 为 `Role` 时引用同 namespace 的 Role；为 `ClusterRole` 时用 ClusterRole。

6.4 绑定中的 subject 可以是 user 或 group。user 与主体用户名精确匹配；group 与 §1 展开后的组集合匹配。

6.5 绑定是否命中只看主体与作用域；规则是否命中按 §2–§5 判断。两层都命中才产生效果。

6.6 对集群级请求（namespace 为空），只使用 ClusterRoleBinding；namespace 内的 RoleBinding 不参与。

## §7 判定缓存

7.1 缓存只允许复用"完全相同的问题"的结论。缓存键必须包含：请求的 user、展开前的组集合、impersonation（若有）、verb、apiGroup、resource、subresource、namespace、name。

7.2 缓存键必须包含计算该结论时所依据的 rolebinding 快照版本。快照版本变化后，旧版本下产生的结论不得再被返回。

7.3 缓存键必须能区分不同主体：用户 A 的 allow 不得被用户 B 在相同资源请求上复用；组集合不同、impersonation 不同的请求也不得互相复用。

7.4 缓存只保存最终结论（allow / deny / no-opinion），不改变判定优先级。

7.5 缓存容量有上限；淘汰策略不得返回已被淘汰条目的结论。

## §8 响应映射

8.1 allow 映射为 `{"allowed": true}`；deny 映射为 `{"denied": true}`；no-opinion 映射为两者皆 false。

8.2 判定理由（reason）不参与授权决定，仅供审计。

8.3 请求缺少 user 或 resource 时，视为格式错误，返回 no-opinion 并给出理由；不得据此放行。
