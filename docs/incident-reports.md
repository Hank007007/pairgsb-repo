# 安全测试记录

本文件收录安全团队在灰度环境上做越权测试时留下的记录。每条记录包含场景、
预期与观察。规范语义见 [relation-model.md](relation-model.md)。

记录编号不代表处理状态，观察结果以当时的构建为准。

---

## IR-01 封禁关系不影响查看权限

**场景**

```
type document {
    relation owner  = self
    relation banned = self
    relation viewer = owner but not banned
}
```

写入：

- `document:handbook#owner@user:alice`
- `document:handbook#banned@user:alice`
- `document:handbook#owner@user:bob`

**预期**（§3.4）

- `Check(user:alice, viewer, document:handbook)` → `false`
- `Check(user:bob, viewer, document:handbook)` → `true`

**观察**

两个 subject 均返回 `true`。把 `banned` 换成只包含 alice 的任意关系，
结论不变：被剔除的关系对结果没有收窄作用。

---

## IR-02 共享目录下的嵌套文档不可见

**场景**

```
type folder {
    relation parent = self
    relation editor = self
    relation viewer = self | editor | viewer from parent
}

type document {
    relation parent = self
    relation editor = self
    relation viewer = self | editor | viewer from parent
}
```

写入：

- `folder:eng#viewer@user:alice`
- `document:spec#parent@folder:eng`
- `document:spec#editor@user:bob`

**预期**（§3.6）

- `Check(user:alice, viewer, document:spec)` → `true`
- `Check(user:bob, viewer, document:spec)` → `true`
- `Check(user:alice, viewer, folder:eng)` → `true`

**观察**

alice 的判定为 `false`，bob 的判定为 `true`。在 `folder:eng` 上额外写入
`folder:eng#parent@user:carol` 之后，`Check(user:carol, viewer, document:spec)`
返回 `true`；而 carol 只持有该目录的 `parent` 授权，在文档与目录上都没有
`viewer`、`editor`、`owner` 中任何一项。

---

## IR-03 深继承链未被截断

**场景**

构造 40 个 folder：`folder:f0` 到 `folder:f39`，`folder:f(i)#viewer@folder:f(i+1)`，
并在 `folder:f39#viewer@user:alice`，schema 为 `relation viewer = self | viewer from viewer`。
对 `folder:f0` 发起 `viewer` 判定。

再把其中两条记录改成互相引用：`folder:a#viewer@folder:b`、
`folder:b#viewer@folder:a`，并对 `folder:a` 发起 `viewer` 判定。

**预期**（§5）

两条路径都应在 25 次下推以内返回 `depth_exceeded`。环不能导致请求无法返回。

**观察**

- 40 层链：返回 `true`，耗时随链长增长。
- 互相引用：请求长时间不返回；在并发压测下工作进程内存持续上涨并最终退出。

---

## IR-04 通配授权放行 subject set

**场景**

```
type document {
    relation viewer = self
}
```

写入：

- `document:pub#viewer@user:*`

对下列请求发起判定：

- `Check(user:anyone, viewer, document:pub)`
- `Check(group:eng#member, viewer, document:pub)`
- `Check(service:ci#runner, viewer, document:pub)`
- `Check(user:alice, viewer, document:pub)`，其中 alice 不是 `group:eng` 的成员

**预期**（§4.2）

- `user:anyone` → `true`
- `group:eng#member` → `false`
- `service:ci#runner` → `false`
- `user:alice` → `true`

**观察**

四个请求全部返回 `true`。把存储中的授权换成 `service:*` 时，
`user:alice` 返回 `false`，但 `group:eng#member` 仍返回 `true`。

---

## IR-05 指定旧版本仍读到最新数据

**场景**

1. 写入 `document:plan#viewer@user:alice`，记录返回的 revision 为 `R`。
2. 写入 `document:plan#viewer@user:bob`。
3. 删除 `document:plan#viewer@user:alice`。
4. 执行 `Check(user:alice, viewer, document:plan)`，请求中携带 `revision = R`。

另在存储执行 GC、水位推进到 `R` 以上之后，携带 `revision = R` 再次判定。

**预期**（§6）

- 第 4 步：在 `R` 的视图上判定，alice 仍为 `true`，返回的 revision 为 `R`。
- GC 之后：返回 `stale_revision`，不得回退到最新视图。

**观察**

- 第 4 步返回 `false`，返回的 revision 等于存储的最新 revision。
- GC 之后携带 `revision = R` 判定，返回 `false` 且 revision 为最新值，
  没有返回任何版本错误。
- 传入一个大于存储当前 revision 的数值，同样不产生错误，结果与不传 revision
  完全一致。
