# argv-policy — 执行计划层的参数与进程边界规范

状态：已签字（安全评审 2025-11-04，owner: platform-security）
适用范围：`internal/runner` 中把插件 step 声明落成 `exec.Cmd` 的全部路径，
以及 `cmd/*` 下任何直接调用该包的入口。

本文件是判定实现是否合格的唯一依据。实现与本文件冲突时，以本文件为准。

术语：

- **step 声明**：`plugin.yaml` 中一个 step 的 `executable`、`args`、`env`、
  `workdir`、`positionals` 字段。
- **渲染**：把 `args` 模板中的 `${INPUT_xxx}` 替换为流水线作者填写的值。
- **最终 argv**：交给 `exec.Cmd.Args` 的切片，`argv[0]` 为可执行文件。
- **基环境**：runner 进程自身继承的环境（`os.Environ()`）。

---

## §1 进程边界：只允许离散 argv，不允许 shell

1.1 最终 argv 必须原样交给 `exec.Cmd`，由操作系统直 `execve`。
实现不得经由 `/bin/sh`、`sh`、`bash`、`cmd.exe`、`powershell` 或任何命令解释器
转发参数，也不得把多个 argv 元素拼成单个字符串再交给解释器。

1.2 `argv[0]` 必须等于 step 声明里的 `executable` 字段的渲染结果。
运行期观察到的进程 argv[0] 不得被 `sh`/`bash` 等解释器替换。

1.3 参数内容一律视为**不透明数据**。含 `;`、`|`、`&`、`$(`、反引号、
`>`、`<`、`*`、换行、引号等字符的参数，与不含这些字符的参数走完全相同的路径；
这些字符没有控制语义，不得触发任何转义、拼接或解释分支。

1.4 禁止任何以"兼容旧插件"为理由的分支绕过 1.1–1.3。需要不同行为的旧插件
必须改写 step 声明。

## §2 占位符展开：单向、一层、一值一元素

2.1 模板中每个 `${INPUT_name}` 至多被替换一次。替换进去的值不再参与后续扫描：
值里出现的 `${INPUT_other}`、`${` 等文本原样保留，不得递归展开。

2.2 一个占位符整体位于一个模板元素内时，渲染结果就是**一个** argv 元素。
值中的空格、制表符不得成为分隔符；不得对渲染结果做空白切分、glob 展开或
二次分词。

2.3 模板元素若由多个占位符和字面量拼接而成（如 `--target=${INPUT_A}`），
拼接结果仍是单个 argv 元素。

2.4 未在 `inputs` 中声明的占位符名，或缺少对应输入值时，`Build` 失败并返回
`ErrUnknownPlaceholder`。错误信息必须包含占位符名，但不得包含任何输入值。

2.5 占位符名允许 `[A-Za-z0-9_]`，区分大小写。`${` 之后没有合法名字与 `}`
的片段按字面量处理，不视为占位符。

## §3 环境变量：allowlist 与唯一性

3.1 最终 env 由两部分组成：基环境中通过 allowlist 的条目、以及 step 声明（即
`StepPlan.Env`）中通过 allowlist 的条目。allowlist 之外的 key 一律丢弃，
静默丢弃即可，但必须在 `BuildResult.DroppedEnv` 中按 key 名记录（去重、稳定排序）。

3.2 allowlist 以本文件附录 A 为准；代码中的集合必须与附录 A 完全一致，
不得额外放行。

3.3 同一个 key 在最终 env 中只允许出现一次。优先级：
`StepPlan.Env` > step 声明 `env` > 基环境。实现必须按 key 覆盖，
不得依赖 `exec.Cmd` 或操作系统的"先出现/后出现"规则。

3.4 基环境中的 `PATH` 属于 runner 自身运行所需，保留在**父进程**环境中；
传给子进程的 env 是否包含 `PATH` 由附录 A 决定。若附录 A 未列出 `PATH`，
则不得下发。

## §4 位置参数与 `--` 分隔符

4.1 若 step 声明包含 `positionals`（含空列表），最终 argv 必须形如：

```
argv[0] = executable
argv[1..n] = 模板渲染出的参数
argv[n+1] = "--"
argv[n+2..] = 位置参数，顺序与声明一致
```

4.2 `--` 只在存在 `positionals` 时出现，且只出现一次，位于所有模板参数之后、
第一个位置参数之前。实现不得把 `--` 拼到任何参数的前面或后面。

4.3 位置参数不做任何"去前导 `-`"的处理：以 `-`、`--` 开头的值必须原样下发，
由 `--` 保证其不被当作选项。实现不得修改位置参数内容。

4.4 若 step 声明不含 `positionals` 字段，则 argv 中不出现 `--`。

## §5 参数合法性校验

5.1 含 NUL（`\x00`）的参数在 `Build` 阶段即被拒绝，返回 `ErrNulInArg`。
错误信息包含参数的序号（从 0 开始，相对最终 argv），不得包含参数内容。
校验失败时不得返回可用的 `exec.Cmd`。

5.2 含裸换行（`\n`）的参数同样在 `Build` 阶段拒绝，返回 `ErrNewlineInArg`，
理由与 5.1 相同：换行会破坏 runner 的事件行协议。

5.3 校验必须覆盖最终 argv 的**每一个**元素，包括 `argv[0]`、`--` 之前与之后的
全部参数。任何一条路径都不得把未校验的参数交给 `exec`。

5.4 `Build` 返回错误时，`BuildResult.Cmd` 必须为 `nil`，`BuildResult.Argv`
可以为空或保留已构造的部分，调用方不得据此启动进程。

## §6 可复现性

6.1 同一份 step 声明与同一组输入，连续调用 `Build` 两次，最终 argv 必须
逐元素相等，最终 env 必须逐条相等（顺序也一致）。

6.2 实现不得依赖 map 迭代顺序、当前时间、随机数或进程 id 来构造 argv/env。
env 的顺序为：先按 key 字典序输出合并后的结果，再按 §3.3 处理覆盖。

## §7 错误码

`Build` 只返回以下哨兵错误（可用 `errors.Is` 判断）：

| 错误 | 含义 | 条款 |
|---|---|---|
| `ErrEmptyExecutable` | executable 渲染后为空 | §1.2 |
| `ErrUnknownPlaceholder` | 未声明或不存在的占位符 | §2.4 |
| `ErrNulInArg` | 参数含 NUL | §5.1 |
| `ErrNewlineInArg` | 参数含裸换行 | §5.2 |

错误只描述位置与类别，不回显输入值。

## §8 工作目录

8.1 `workdir` 渲染后若非空，必须为绝对路径或相对于 runner 工作目录的路径，
`Build` 不做存在性检查（由 runner 在执行期报告）。
8.2 `workdir` 同样经过 §2 的占位符则与 §5 的字符校验。

## §9 与 runner 的契约

9.1 `Build` 必须在返回 `exec.Cmd` 之前完成全部校验与渲染；runner 拿到
`*exec.Cmd` 后不再修改 `Args`、`Env` 或 `Dir`。

9.2 `BuildResult.Argv` / `BuildResult.Env` 是审计副本，必须与 `exec.Cmd`
中的内容一致。

9.3 runner 记录审计日志时只能记录 key 名与参数序号，不得记录参数值。

---

## 附录 A — 环境变量 allowlist

允许下发给子进程的 key（大小写敏感，不做前缀匹配）：

```
CI
CI_PIPELINE_ID
CI_STEP_ID
HOME
LANG
LC_ALL
NO_COLOR
TZ
TMPDIR
```

显式**不**允许（示例，非穷举）：`PATH`、`LD_PRELOAD`、`LD_LIBRARY_PATH`、
`GIT_SSH_COMMAND`、`NODE_OPTIONS`、`PYTHONPATH`、`BASH_ENV`、`IFS`、
`SHELL`、`ENV`。不在上表内的 key 一律视为不允许。
