# envseal

envseal 是 CI runner 旁的密钥注入 sidecar。job 开跑时，agent 通过本地 unix socket 申请一份环境变量包；envseal 按策略校验、从加密的 secret store 解密、生成只有该 job 用户可读的 env 文件，并在 job 结束或 lease 到期时回收。同时，job 的 stdout/stderr 会经过 envseal，已知 secret 值在转发给前端之前被抹掉。

## 目录

```
bin/envseald.js        进程入口
lib/app.js             HTTP 服务与依赖装配
lib/policy.js          策略匹配与 scope 合并
lib/lease.js           lease 生命周期
lib/envfile.js         env 文件渲染 / 写入 / 删除
lib/mask.js            secret 匹配器与掩码
lib/stream.js          日志字节流的 Transform
lib/store.js           加密 secret store
routes/api.js          HTTP 路由
docs/                  规范与设计说明
testdata/              策略、流水线声明、secret store
test/                  node:test 用例
```

## 运行

```sh
node bin/envseald.js \
  --socket ./run/envseald.sock \
  --leases-dir ./run/leases \
  --policy ./testdata/policies/default.json \
  --pipelines ./testdata/pipelines \
  --store ./testdata/store.json \
  --store-key ./testdata/store.key
```

Windows 上 `--socket` 会作为 named pipe 路径使用。

## 测试

```sh
node --test
```

## 文档

- [docs/lease.md](docs/lease.md) — lease 的申请、复用、到期与撤销
- [docs/injection.md](docs/injection.md) — env 文件的权限、合并与转义
- [docs/masking.md](docs/masking.md) — 日志掩码规则
- [docs/policy.md](docs/policy.md) — 策略匹配与 scope 优先级
- [docs/design.md](docs/design.md) — 两条主链路的设计说明
