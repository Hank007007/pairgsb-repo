# ldapfence

公司目录服务与 LDAP 之间的中间层。员工登录、组织架构同步、运维高级搜索
三类请求都从这里出网，最终变成一条过滤器或一个 DN 交给 LDAP 客户端。

```
bin/ldapfence.js     命令行入口（基于进程内 mock 客户端）
src/auth.js          登录：查找账号、拼装绑定 DN、bind
src/filter.js        过滤器值的转义与常见过滤器构造
src/dn.js            DN 值的转义与绑定 DN 构造
src/search.js        搜索选项归一化、分页、按 ou / 服务账号的同步
src/advanced.js      管理员手写过滤器的解析入口与高级搜索
src/ldap/            过滤器语法树、DN 解析、mock 客户端与目录数据
docs/filter-escaping.md   边界规范：正确行应当是什么
test/                正常路径的回归用例
```

## 运行

要求 Node.js 20+，只使用标准库，无需安装依赖。

```bash
node --test
node bin/ldapfence.js lookup alice
node bin/ldapfence.js sync engineering 2
node bin/ldapfence.js advanced '(title=Engineer)'
node bin/ldapfence.js login alice alice-secret
```

`createMockClient()` 是与真实客户端同形的进程内实现：`client.search(base, options)`
与 `client.bind(dn, password)`，目录数据见 `src/ldap/seed.js`。它把每次出网调用
记录在 `client.operations` 上，便于观察本层到底发出了什么。真实部署时由调用方
注入 ldapjs 客户端，本层不感知协议细节。

## 边界契约

见 [`docs/filter-escaping.md`](docs/filter-escaping.md)。概要：

- 用户值进入过滤器前按 RFC 4515 转义（`\ * ( )` 与 NUL）；
- 用户值进入 DN 前按 RFC 4514 转义，并处理开头 `#` 与首尾空格；
- `base` 必须位于配置的 suffix 之下，`scope` 只能是 `base` / `one` / `sub`；
- 高级搜索过滤器先过语法树，再检查属性黑名单与恒真结构。

## 配置

`createConfig()` 见 `src/config.js`：`suffixes`、`peopleSuffix`、`serviceSuffix`、
`search`、`advanced`。默认值指向 `dc=corp,dc=example` 下的 `ou=people` 与
`ou=service`。
