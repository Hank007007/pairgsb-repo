# cborveil 设计说明

本文描述设备端 OTA manifest 解析库的目标行为。规范条款见
[`docs/manifest-cbor.md`](docs/manifest-cbor.md)，本文说明各模块的职责边界以及
「正确的实现应当满足什么」，不涉及具体实现方式。

## 1. 模块划分

| 包 | 职责 |
|---|---|
| `internal/cbor` | 通用 CBOR 子集解码、规范化判定、编码 |
| `internal/manifest` | manifest 结构映射、待签字节计算、验签、目标选择 |
| `cmd/otadump` | 命令行 dump 工具，输出格式稳定 |

数据流是单向的：

```
字节 ──IsCanonical──▶ Decode ──parse──▶ Manifest ──SigningBytes──▶ Verify
                                                  └── Select ──▶ Target
```

`Parse` 是设备侧唯一的入口，它必须保证：任何通过 `Parse` 返回的 `Manifest` 都已
满足规范第 2、3、8 节的字节级约束，以及第 4、5 节的结构约束。`Verify` 和
`Select` 只在该前提下工作。

## 2. 规范化判定

判定对象是**原始字节**，不是解码后的结构。因为一旦先解码再判定，键顺序、整数
编码长度、indefinite-length 等信息就丢失了。

正确行为：

- 递归遍历整棵数据项树，每一个 map（不论嵌套多深、位于 array 元素还是 map 值）
  都必须按键的编码字节严格升序。
- 整数必须在任意出现位置都是最短形式；判定应基于初始字节的附加信息，而不是
  解码后的数值。
- indefinite-length 的任何形式都不得出现。
- 判定失败与「不是单个完整数据项」必须区分于「合法」结果：`IsCanonical` 为
  假时 `Parse` 直接拒绝。

## 3. 解码

解码器接受规范第 2 节列出的类型。正确行为：

- 遇到不支持的编码（indefinite-length、tag、浮点）时，在**读到该初始字节时**
  即报错，不得继续把它当作普通数据项消费。
- 嵌套深度限制必须在所有递归分支上生效：array 的元素、map 的键、map 的值、
  以及任何组合。深度计数以顶层为 1。
- 重复键的语义由规范第 9 节定义：未知键不得覆盖已知字段。解码层可以提供多值
  承载方式，但 `Parse` 的最终结果必须只反映规范允许的那一种解释。
- 输入必须被完整消费。

## 4. manifest 映射

`Parse` 把解码树映射为 `Manifest`。正确行为：

- 顶层与每个 target 的必选键、类型、长度约束按规范第 4、5 节逐一检查。
- 整数转换必须检测越界：CBOR 的无符号 64 位取值超出 `int64` 范围时报错，而不是
  截断或回绕。
- `targets` 内每个字段都要做值与长度校验：`url` 非空、`sha256` 恰为 32 字节、
  `size` 大于 0、`offset+size` 不超过整包长度。
- 同一 `model` 在一个 manifest 内出现多次时整份 manifest 非法。

## 5. 待签字节

`SigningBytes` 的输入是解析后的 manifest，输出必须只由 manifest 的**语义**决定。

正确行为：

- 待签内容是顶层 map 去掉 `sig` 与 `key_id` 之后的全部键值对，包括未知键。
- 输出必须是规范化编码，且 map 键按编码字节排序，与 manifest 在传输时的键顺序
  无关。
- 同一语义的两份 manifest（仅键顺序或整数编码不同）必须得到逐字节相同的输出。
- 签名端与验签端共用这一个函数，不得各自重新编码。

## 6. 目标选择

正确行为：

- 机型匹配是完整相等，不做任何前缀/模糊匹配。
- 零个匹配返回「无适用分片」错误；多于一个匹配返回「manifest 非法」错误。
- 选择结果只依赖 `model` 字段，不依赖数组顺序。

## 7. 资源与环境

- 单个 manifest 上限 1 MiB，超限在解析前返回错误。
- 深度上限 16。
- 只使用 Go 标准库；Ed25519 验签使用 `crypto/ed25519`。
- 所有错误都向上返回，不 panic，不返回部分结果。

## 8. dump 工具

`otadump` 的输出是稳定的逐行格式，字段顺序固定：

```
version: <string>
size: <integer>
key_id: <string>
sig: <N> bytes
targets: <N>
target[i]: model=<...> url=<...> sha256=<hex> size=<...> offset=<...>
[selected: ...]
```

错误写到 stderr，以非零状态退出。解析失败时不得输出任何 target 行。
