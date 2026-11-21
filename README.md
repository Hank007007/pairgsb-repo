# schemabind-gate

API 网关的请求校验层：租户上传 JSON Schema，网关编译成校验计划，
对请求体做校验与类型归一，通过后把归一实例交给下游。

## 目录

```
src/schema/refs.js       $ref 解析、远端抓取、环检测
src/schema/compile.js    schema → 校验计划、正则与编译缓存
src/validate/coerce.js   标量归一、default 写入
src/validate/walk.js     计划驱动实例校验
src/http/gateway.js      validateRequest 入口与错误组装
src/http/server.js       HTTP 路由
src/policy/limits.js     常量与正则策略
docs/                    设计口径
```

## 运行

```bash
node --test
```

## 规范

设计口径以 `docs/` 为准：

- `docs/dialect.md` — schema 方言、关键字支持面、`additionalProperties` 缺省口径
- `docs/refs.md` — `$ref` 解析、远端白名单、抓取约束、环检测身份
- `docs/coercion.md` — 逐类型归一对照表、default 语义
- `docs/limits.md` — 请求体上限、正则策略、嵌套深度、编译缓存
- `docs/design.md` — 四个阶段各自的正确行为与不变式

对外函数签名固定：`compileSchema(schema, options)`、
`resolveRef(rootSchema, ref, options)`、`coerceScalar(value, type)`、
`applyDefaults(plan, instance)`、`validateRequest(schema, body, options)`。