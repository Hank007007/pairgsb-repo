# stowsign

为对象存储签发和校验预签名 URL 的 Python 库。只依赖标准库。

签发端拿调用方的策略生成一条带 `X-Stow-Algorithm`、`X-Stow-Expires`、
`X-Stow-Signature` 等查询参数的 URL；校验端把请求原样重算一遍签名再比对。
策略可以限制 HTTP 方法、资源路径前缀、来源 IP/CIDR、必须签名的 header，
并支持服务端复制（source + target 两个资源）。

签名规范见 [`docs/presign-spec.md`](docs/presign-spec.md)，设计说明见
[`docs/design.md`](docs/design.md)，历史线上事件见
[`docs/incident-reports.md`](docs/incident-reports.md)。

## 安装与测试

```bash
python -m pytest -q -p no:cacheprovider
```

无需安装任何第三方依赖；`pytest` 仅用于跑测试。

## 快速上手

```python
from stowsign import Policy, presign, verify
from urllib.parse import urlsplit

policy = Policy.from_dict({
    "version": 1,
    "methods": ["GET"],
    "resources": ["photos"],
    "ip_cidrs": ["10.0.0.0/8"],
    "signed_headers": [],
})

grant = presign(policy, "secret", "GET", "photos/cat.jpg", ttl=600)

parts = urlsplit(grant.url)
result = verify(
    "GET",
    parts.path,
    parts.query,
    "secret",
    policy=policy,
    client_ip="10.1.2.3",
)
print(result.resource)  # photos/cat.jpg
```

### 服务端复制

```python
grant = presign(
    policy, "secret", "PUT",
    "photos/cat.jpg",
    target="backup/cat.jpg",
)
```

`X-Stow-Target` 只在复制场景出现；源与目标都必须落在策略覆盖的范围内。

## 模块

| 模块 | 职责 |
|---|---|
| `stowsign/canonical.py` | 百分号编码、规范查询串、规范 header 块、规范请求、HMAC |
| `stowsign/policy.py` | 策略解析与条件判定（方法 / 资源 / 来源网络 / 必需 header） |
| `stowsign/grant.py` | 签发：校验策略、冻结截止时间、拼接 URL |
| `stowsign/verify.py` | 校验：算法、有效期、header 绑定、策略、签名比对 |
| `stowsign/clock.py` | 时间戳与有效期判定 |
| `stowsign/errors.py` | 异常类型 |

## 异常

所有失败都派生自 `stowsign.StowSignError`：

- `PolicyError` — 请求落在策略之外。
- `SignatureError` — 签名或 header 绑不成立。
- `ExpiredError` — 已过截止时间。

## 配置项

- `presign(..., ttl=...)`：授权时长，默认 `3600` 秒。
- `presign(..., endpoint=...)`：URL 前缀，默认 `https://storage.example.com`。
- `verify(..., now=...)`：注入当前时刻，便于测试。
