# dialect.md — 网关接受的 schema 方言

schemabind 并不是一份完整的 JSON Schema 实现。租户在控制台上传的 schema 会先经过
一个固定子集的检查，只有落在本文件描述的方言里的关键字才会被编译进校验计划。

## §1 对象与关键字

### §1.1 顶层形状

顶层 schema 必须是一个 JSON 对象（`type: "object"` 的子集即可），
带 `$id` 时仅作为诊断用途，不参与引用解析。

### §1.2 支持的 `type`

`object`、`array`、`string`、`number`、`integer`、`boolean`、`null`。
不支持的类型名在编译期被拒绝。

### §1.3 支持的关键字

对象：`type`、`properties`、`required`、`additionalProperties`、
`patternProperties`、`minProperties`、`maxProperties`。

数组：`type`、`items`、`minItems`、`maxItems`、`uniqueItems`。

标量：`type`、`enum`、`const`、`pattern`、`patternFlags`、
`minimum`、`maximum`、`exclusiveMinimum`、`exclusiveMaximum`、
`multipleOf`、`format`（仅 `date-time`、`email`、`uuid` 三种）。

公共：`$ref`、`$defs`、`definitions`、`default`、`description`、`title`、
`oneOf`、`anyOf`、`allOf`。

未列出的关键字在编译期被忽略，不是错误。

### §1.4 `additionalProperties` 的缺省口径

JSON Schema 规范里 `additionalProperties` 缺省等价于 `true`。
**网关口径不是这样**：在 schemabind 里，`additionalProperties` 缺省等价于
`false`。也就是说，一个 `properties` 里没声明、`patternProperties` 也没命中的
字段，在缺省配置下就是未知字段，整条请求按校验失败处理。

租户要放行未知字段必须显式写 `additionalProperties: true`。
把 `additionalProperties` 写成 schema 对象时，它的语义与标准一致：未知字段
逐个按该 schema 校验。

## §2 `patternProperties` 与 `additionalProperties` 的判定顺序

对对象实例里的每一个字段，按下面的顺序判定，**一旦命中就停止**：

1. 字段名在 `properties` 里 → 用对应子 schema 校验。**不再**看
   `patternProperties`，也**不再**看 `additionalProperties`。
2. 字段名匹配某条 `patternProperties` 的正则 → 用该条子 schema 校验。
   多条同时匹配时全部都要通过。**不再**看 `additionalProperties`。
3. 以上都不命中 → 交给 `additionalProperties` 处理（缺省按 §1.4 视为拒）。

`patternProperties` 先于 `additionalProperties` 判定，这一点决定了
「一个字段既匹配 pattern 又被 additional 拒掉」不会发生——只要 pattern 命中，
additional 就不参与。

## §3 `oneOf` / `anyOf` / `allOf`

- `anyOf`：至少一条分支通过即可，全部通过也不算错。
- `oneOf`：恰好一条分支通过。0 条通过和 ≥2 条通过都算失败。
- `allOf`：所有分支都必须通过。

分支校验互不影响实例本身：分支看到的是同一份实例快照，
分支内部的 `default` 不写回，分支的失败不污染成功分支。

## §4 错误结构

校验失败时对外返回的错误条目形如：

```
{ "path": "/a/b/0/c", "keyword": "required", "message": "..." }
```

`path` 是 JSON Pointer，指向实例里出问题的位置。分支型关键字的错误
`path` 指向分支内部实际出错的位置，而不是最外层。

错误体里不得出现实例的值（`instance` 原文）、远端引用的完整 URL、
以及任何堆栈。需要定位 `$ref` 时只给它在提交的 schema 里的位置。
错误信息面向调用方，不面向网关内部的调试。