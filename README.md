# brokeracl

MQTT 风格消息 broker 的内嵌 ACL 插件。按 principal 组织规则，回答三个问题：

- 这个客户端能不能向某个 topic **发布**？
- 这个客户端能不能**订阅**某个 topic filter？
- 这个客户端此刻能不能**收到**发往某个 topic 的消息？

默认拒绝；规则目录下所有 `*.json` 合并生效；`SIGHUP` 触发整体热加载。

## 文档

- [`docs/acl-rules.md`](docs/acl-rules.md) — 规范与设计说明，定义正确行为（规范性文档）
- [`docs/incident-reports.md`](docs/incident-reports.md) — 线上事件记录与规范依据

## 目录

```
acl/                  样例规则文件（按文件名字典序合并）
docs/                 规范与运维文档
src/acl/
  parse.js            读取、校验、合并规则文件
  match.js            filter 语法、filter 匹配与覆盖关系
  evaluate.js         身份到 principal 的映射、单条与整体结论
  reload.js           ACL 实例、规则替换、SIGHUP 处理
  share.js            共享订阅前缀拆分
src/broker/
  subscribe.js        订阅鉴权
  deliver.js          在线投递与 retained 投递
  will.js             遗嘱发布
  index.js            内存 broker 门面
test/                 node:test 用例
examples/             可运行示例
```

## 运行

需要 Node.js 20 及以上，无第三方依赖。

```bash
node --test
```

示例：

```bash
node examples/walkthrough.js
```

## 规则文件格式

```json
{
  "principals": {
    "user:alice": {
      "rules": [
        { "action": "publish",   "topic": "sensors/+/state", "effect": "allow" },
        { "action": "subscribe", "topic": "sensors/#",       "effect": "allow" }
      ]
    }
  }
}
```

principal key 前缀：`cn:`（证书 CN）、`user:`（用户名）、`client:`（clientId）。
一个连接可同时映射到多个 principal，结论取并集；详见规范 §2。

## 热加载

```js
import { createAcl, installReloadHandler } from './src/acl/index.js';

const acl = createAcl({ rules: { principals: {} } });
const handler = installReloadHandler({ acl, directory: './acl' });
// 进程收到 SIGHUP 时重新读取 ./acl 下所有 *.json
```

## 库入口

- `src/acl/index.js`：`createAcl`、`evaluate`、`decide`、`parseRuleset`、`loadAclDirectory`、
  `reloadAcl`、`installReloadHandler`、`filterCovers`、`topicMatchesFilter`
- `src/broker/index.js`：`createBroker`（内含 `onSubscribe` / `deliverMessage` / `deliverRetained` / `publishWill`）
