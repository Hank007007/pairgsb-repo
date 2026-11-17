# keyladder

API 网关后面的密钥生命周期服务：签发、验证、轮换、收回。

```bash
node --test
node bin/keyladder.js demo
```

- 为规范：[`docs/key-policy.md`](docs/key-policy.md)
- 结构与组件：[`docs/design.md`](docs/design.md)

## 快速上手

```bash
node bin/keyladder.js issue --owner checkout-team --scopes orders:read,orders:write --env prod
node bin/keyladder.js verify kl_prod_<id8>_<secret> --env prod
node bin/keyladder.js rotate <id8>
node bin/keyladder.js revoke <id8>
```

数据文件默认为 `data/keys.json`，可用 `--data <file>` 或 `KEYLADDER_DATA_FILE` 指定。

生产部署请设置 `KEYLADDER_PEPPER`。仓库内的默认 pepper 只用于本地开发。

## 目录

```
bin/        命令行入口
src/        服务实现
test/       node:test 用例
docs/       规范与设计说明
data/       示例数据文件
```
