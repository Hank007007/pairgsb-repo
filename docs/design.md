# argvsieve-exec-args 设计说明

本仓库是内部 CI runner 的「执行计划」层。它把插件在 `plugin.yaml` 里声明的
一个 step（`executable`、`args` 模板、`env`、`workdir`、`positionals`），加上
流水线作者填写的 `${INPUT_xxx}` 输入值，落成一个可以直接交给 runner 启动的
`exec.Cmd`。

安全评审签字的规范是 [`docs/argv-policy.md`](argv-policy.md)。本文叙述的是
**正确的行为应当是什么**，也就是实现规范对齐之后应当具备的可观察性质；
本文不描述实现方式，也不覆盖历史行为。

---

## 1. 分层与数据流

```
plugin.yaml ──► internal/plugin  解析成 plugin.Step
                       │
pipeline 输入 ─────────┤
                       ▼
                 internal/runner.Build(StepPlan) ──► BuildResult{Cmd, Argv, Env, DroppedEnv}
                       │
                       ▼
                 runner 启动 exec.Cmd
```

- `internal/plugin` 只负责把清单读成结构体，不解释任何占位符语义。
- `internal/runner.Build` 是唯一的渲染与校验入口，返回前必须已经完成全部
  渲染、env 合并和参数校验；返回之后 `exec.Cmd` 不再被修改（§9.1）。
- `BuildResult.Argv` / `BuildResult.Env` 是审计副本，必须与 `exec.Cmd` 中的
  内容一致（§9.2）。

`StepPlan` 与 `BuildResult` 是稳定的对外契约：

- `StepPlan` 的字段只增不改，构造函数不引入新的必填项；零值可用，
  只有 `Executable` 需要由调用方提供，`BaseEnv` 留空时取 `os.Environ()`。
- `Build(StepPlan) (BuildResult, error)` 的签名固定。
- 新增行为（例如记录被丢弃的 env key）通过已有的 `DroppedEnv` 字段表达，
  不新增必填。

## 2. 渲染：单向、一层、一值一元素

模板中的占位符写作 `${INPUT_name}`，`name` 为 `[A-Za-z0-9_]+`，区分大小写。
不在 `inputs` 中的名字使 `Build` 返回 `ErrUnknownPlaceholder`（§2.4）。

渲染结果必须满足：

1. **一层**：替换进去的值不再被扫描。值里若含 `${INPUT_other}`、`${` 等
   文本，原样保留（§2.1）。
2. **一值一元素**：一个模板元素渲染出的字符串就是一个 argv 元素，值里的
   空格、制表符、引号、通配符都不产生新的分隔（§2.2）。
3. **拼接仍是一个元素**：`--target=${INPUT_A}` 这类多段模板同样只产生一个
   argv 元素（§2.3）。
4. `${` 之后不构成合法引用的字符序列按字面量处理（§2.5）。

由此，`INPUT` 的值是纯数据：它能不能改变 argv 的长度、能不能改变
`argv[0]`、能不能让别的程序被启动，答案都必须是"不能"。

## 3. 进程边界

最终 argv 直接交给 `exec.Cmd`，由操作系统 `execve`，中间不经过任何命令
解释器（§1.1）。因此：

- `argv[0]` 恒等于 step 声明的 `executable` 渲染结果（§1.2）。
- `;`、`|`、`&`、`$(`、反引号、重定向、通配符等字符没有任何控制语义，
  含这些字符的参数与普通参数走同一条路径（§1.3）。
- 不存在"遇到特殊字符就换一种执行方式"的分支；兼容旧插件不能成为绕过
  这条边界的理由（§1.4）。旧插件应当把一条复合命令改写成若干离散参数。

## 4. 环境变量

环境变量按 allowlist 过滤（附录 A），基环境与 step 声明中不在表内的 key
一律丢弃，并在 `DroppedEnv` 中记录 key 名（去重、稳定排序，§3.1）。

同一个 key 在最终 env 中只出现一次，优先级为 step 声明 > 基环境（§3.3）；
不允许依赖"先出现/后出现"或平台选择规则。

顺序固定：合并后按 key 字典序输出（§6.2），保证两次 Build 的 env 逐条相等。

## 5. 位置参数与 `--`

存在 `positionals` 字段（哪怕是空列表）时，最终 argv 形如：

```
executable, <模板参数...>, "--", <位置参数...>
```

`--` 恰好出现一次，位于所有模板参数之后、第一个位置参数之前（§4.1、§4.2）。
位置参数的内容不被修改，以 `-` 开头的值依靠 `--` 获得选项语义豁免，
而不是靠删掉前缀（§4.3）。声明里没有 `positionals` 时 argv 中不出现 `--`
（§4.4）。

## 6. 校验

`Build` 在返回可用 `exec.Cmd` 之前校验终 argv 的每一个元素（含 `argv[0]`
和 `--` 两侧）：

- 含 NUL 的参数返回 `ErrNulInArg`（§5.1）。
- 含裸换行的参数返回 `ErrNewlineInArg`（§5.2）。

错误信息只带参数序号和类别，不回显参数值（§7）。校验失败时 `Cmd` 必须为
`nil`（§5.4），调用方不得启动进程。

## 7. 错误码

| 错误 | 含义 |
|---|---|
| `ErrEmptyExecutable` | executable 渲染后为空 |
| `ErrUnknownPlaceholder` | 未声明或不存在的占位符 |
| `ErrNulInArg` | 参数含 NUL |
| `ErrNewlineInArg` | 参数含裸换行 |

全部可用 `errors.Is` 判断，不新增其它哨兵错误。

## 8. 可复现性

同一份声明、同一组输入，连续两次 `Build` 得到的 argv 逐元素相等、env 逐条
相等（§6.1）。实现不得依赖 map 迭代顺序、时间、随机数或进程 id。

## 9. 清单格式

`plugin.yaml` 支持缩进式 YAML 的一个小子集：映射、标量序列、引号标量、
整行注释。约定：

- `steps` 为映射，键是 step 名。
- 每个 step 的 `executable` 必填；`args`、`env`、`positionals` 为标量序列；
  `env` 条目写作 `KEY=VALUE`。
- `positionals` 字段"存在"与"为空"是两种不同声明，前者会在 argv 中带上
  `--`。

示例见 [`examples/plugins/demo/plugin.yaml`](../examples/plugins/demo/plugin.yaml)。

## 10. 入口与验证

- `cmd/demo`：从清单构建计划并可选执行；`-twice` 做两次构建对比，
  `-run` 真正启动进程。它会在需要时把 `cmd/echoargs` 构建成 `./bin/echoargs`。
- `cmd/echoargs`：把收到的 argv、cwd、少量 env 打印出来，便于肉眼核对。
- `internal/runner/exec_plan_baseline_test.go`：正常路径的基线用例。
- `internal/runner/testdata/`：样例插件清单。


验收口径见仓库根目录的 `README.md`。
