# sharefence

内部文件协作后端的授权层：在文件树上解析 ACL，并校验发往外部的一次性分享链接。

## 目录

```
src/acl/resolve.js    解析某主体在某节点上的实际角色
src/acl/roles.js      角色合法性高低比较
src/acl/list.js       按调用方可见性过滤目录
src/path/canonical.js 路径规范化与子树判断
src/share/link.js     分享链接的签发、校验、撤销
src/store/memory.js   节点 / ACL 条目 / 组 / 链接的内存存储
docs/acl-model.md     行为契约（正确行为以此为准）
docs/incident-reports.md  两次外链事故的记录
examples/demo.js      公开 API 的最小演示
test/                 node:test 用例
```

## 运行

只需要 Node.js 20 及以上，无第三方依赖。

```sh
node --test        # 运行测试
npm test           # 同上
node examples/demo.js
```

## 公开接口

`src/index.js` 汇总导出全部公开函数，详见 `docs/acl-model.md` §9。

```js
import {
  createStore, addNode, addAcl, addGroup,
  effectiveRole, hasRole,
  visibleChildren,
  normalize, isInside,
  issueLink, verifyLink, revokeLink,
} from './src/index.js';
```
