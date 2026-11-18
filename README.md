# verdictmesh

内嵌式授权决策引擎（PDP）。读入一组 JSON 策略规则，对 `{ principal, resource, action, context }` 返回 `{ allow, matchedPolicy, reason }`。

- 行为规范：[`docs/policy-semantics.md`](docs/policy-semantics.md)
- 设计说明：[`docs/design.md`](docs/design.md)
- 接入方反馈：[`docs/usage-notes.md`](docs/usage-notes.md)

## 运行

```bash
node --test          # 跑测试
node examples/decide.js   # 跑一个端到端示例
```

无需安装依赖，只使用 Node 内置模块（Node.js >= 20）。

## 快速开始

```js
import { createEngine } from './src/index.js';

const engine = await createEngine({ policyDir: './policies' });

const result = engine.decide({
  principal: '/users/alice',
  resource: '/docs/reports/q3',
  action: 'read',
  context: { role: 'member' },
});

console.log(result);
```

批量：

```js
const results = engine.decideMany([
  { principal: '/users/alice', resource: '/docs/a', action: 'read', context: { role: 'member' } },
  { principal: '/users/alice', resource: '/billing/invoices/42', action: 'read', context: { role: 'member' } },
]);
```

## 策略文件

`policies/` 下每个 `.json` 是一组规则（数组，或 `{ "rules": [...] }` 对象）：

```json
[
  {
    "id": "docs-member-read",
    "principal": "/users/*",
    "resource": "/docs/**",
    "action": ["read"],
    "condition": { "role": "member" },
    "effect": "permit"
  }
]
```

字段与语义见规范文档。

## 目录

```
src/          引擎实现
policies/     样例策略
docs/         规范、设计与接入反馈
test/         node:test 测试
examples/     可执行示例
```
