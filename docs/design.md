# certbrace 设计说明

certbrace 是运维平台内部的 SSH 证书签发服务：调用方提交申请者的公钥和申请单，
服务按申请单上的 role 决定证书的 principal、有效期和 critical options，用 CA
私钥签出一张 OpenSSH 用户证书。签发的正确行为以
[`sshca-spec.md`](./sshca-spec.md) 为准，本文只说明系统的组成、边界和数据流。

## 1. 组件与职责

```
            申请单 + 公钥
                 │
                 ▼
        ┌─────────────────┐
        │  lib/issue.js   │  组装证书字段、确定 serial、调用签名
        └────────┬────────┘
     ┌───────────┼──────────────┐
     ▼           ▼              ▼
 lib/policy.js  lib/ca.js   lib/format.js
 role → 允许    选活跃 CA、   OpenSSH wire format
 principal/TTL  签名         编解码
                 │
                 ▼
           lib/verify.js      用 CA 公钥自检签名与有效期
                 │
                 ▼
        证书文本 + 自检报告
```

- `lib/policy.js` — 纯数据与查询函数。role 到允许 principal 集合、默认有效期的
  映射表，以及全局有效期上限。它不做请求校验，也不持有状态。
- `lib/format.js` — 与外部世界（sshd、ssh-keygen）之间的格式边界。所有 SSH
  wire format 的编解码都收敛在这里：string、uint32、uint64、name-list、
  option 列表，以及证书的组装与解析。其余模块只操作普通对象和 Buffer。
- `lib/ca.js` — 与磁盘上 CA 材料的边界。负责枚举 `ca/`、加载 PEM、从私钥
  派生公钥并编码成 OpenSSH blob，以及调用签名原语。key id 的命名约定
  （`<kid>.key`、`.retired` 后缀）在这一层解释。
- `lib/issue.js` — 签发流程的编排：规范化请求、按 role 解析 principal 与
  有效期、收集 critical options、分配 serial、选择 CA、签名、触发自检。
  它返回结构化结果，不负责输出格式。
- `lib/verify.js` — 自检。重新解析已编码的证书，独立于签发路径验证签名和
  有效期窗口，产出可展示的报告。它不修改证书，也不参与授权决策。
- `lib/errors.js` — 统一的错误类型，携带稳定的错误码与 HTTP 语义的状态码，
  便于 CLI 和将来的 HTTP 层复用。
- `bin/certbrace.js` — 命令行入口，读 JSON 申请单、调用签发流程、输出证书
  和自检摘要。

## 2. 数据流

1. 调用方给出申请单对象（字段见规范第 1 节）和可选参数（CA 目录、serial
   状态文件、时钟）。
2. `issue.js` 先做结构规范化与 role 存在性检查，再把公钥解析成 OpenSSH blob。
3. principal 集合、有效期、critical options、serial 依次确定，随后选定 CA
   私钥。
4. `format.js` 把字段编码成“证书主体”，`ca.js` 对主体签名，`format.js` 追加
   签名得到完整证书 blob，再渲染成 authorized_keys 行。
5. `verify.js` 从证书文本重新解析并自检，报告随结果返回。

任何一步校验失败都抛出 `CertbraceError`，不产生证书。自检结果不改变已经
签出的证书，但必须如实呈现在结果中。

## 3. 状态

- **CA 材料**：`ca/` 下的 PEM 私钥，按 `<kid>.key` 命名；对应的
  `<kid>.pub` 提交给 sshd 的 `TrustedUserCAKeys`。轮换通过新增 key 并给旧
  key id 加 `.retired` 标记完成，目录内容即为全部状态。
- **serial 状态**：默认位于 `var/serial.seq`，只保存“已分配到的最后一个
  serial”。该文件属于运行时状态，不入库。CLI 与测试可指向别处。
- 服务不保存已签发证书本身；证书是 bearer credential，由调用方持有。

## 4. 时钟与单位

对外的申请单以秒为单位，`Date.now()` 返回毫秒，证书 wire format 里的
`valid_after` / `valid_before` 同样是秒。三种表示之间的换算由调用边界
负责，跨模块传递的字段始终携带自身定义的单位。有效期上限（规范第 3 节）
与自检（规范第 7 节）都以证书中的秒值为准。

## 5. 测试策略

测试用 `node:test` 与 `node:assert/strict`，不依赖外部进程或固定凭据：

- CA 密钥在测试内用 `crypto.generateKeyPairSync('ed25519')` 现场生成，写入
  临时目录；
- 申请者密钥同样现场生成；
- 断言覆盖正常路径：role 的默认 principal 与 TTL、`force_command` 编码、
  CA 选择、签名验证、证书解析往返。

需要人工核对 wire format 时，把证书写到文件后使用
`ssh-keygen -L -f <file>` 对照规范第 4 节。

## 6. 非目标

- 不做 HTTP 服务、鉴权、审计落库；这些由平台侧承担。
- 不支持 Ed25519 以外的密钥类型。
- 不实现 KRL 生成；serial 的意义在于让外部吊销机制可以精确定位证书。
- 不管理 `ca/` 的轮换过程本身，轮换由基础设施组落地为目录中的文件变化。
