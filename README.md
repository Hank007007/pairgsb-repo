# sigvault

内部对象存储前面的签发服务：业务方用服务凭据换一条带签名的 URL，客户端拿着 URL
直接向存储上传或下载，无需再经过业务服务。纯标准库实现。

```
sigvault/
  sign.py     规范串与 HMAC-SHA256 签名
  verify.py   签名与有效期校验
  policy.py   key 的授权范围
  store.py    本地目录存储
  app.py      HTTP 入口与请求编排
docs/
  presign-spec.md   规范性说明
tests/              正常路径的回归测试
```

## 运行

```console
$ python -m sigvault --root ./data \
    --service 'demo=s3cr3t:d3m0-t0ken:team-a,team-b:1048576'
sigvault listening on http://127.0.0.1:8080 (root=./data)
```

## 签发

```console
$ curl -s http://127.0.0.1:8080/presign \
    -H 'Authorization: Bearer d3m0-t0ken' \
    -H 'Content-Type: application/json' \
    -d '{"bucket":"assets","key":"team-a/logo.png","method":"PUT",
         "content_type":"image/png","max_size":65536,"expires_in":120}'
{"content_type":"image/png","expires_at":1763000000,"max_size":65536,
 "method":"PUT","url":"/assets/team-a/logo.png?svc=demo&exp=1763000000&sig=...&max-size=65536"}
```

## 上传与下载

```console
$ curl -X PUT 'http://127.0.0.1:8080/assets/team-a/logo.png?svc=demo&exp=...&sig=...&max-size=65536' \
    -H 'Content-Type: image/png' --data-binary @logo.png
{"size":8192,"stored":"assets/team-a/logo.png"}

$ curl 'http://127.0.0.1:8080/assets/team-a/logo.png?svc=demo&exp=...&sig=...' -o logo.png
```

## 测试

```console
$ python -m pytest -q -p no:cacheprovider
```

## 规范

签名覆盖哪些字段、key 如何授权、路径如何解析、超限上传如何处理，均以
[`docs/presign-spec.md`](docs/presign-spec.md) 为准。
