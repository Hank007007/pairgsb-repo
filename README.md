# formpost

工单系统前端提交表单的解析层。它接收 `multipart/form-data` 请求体，拆成
若干字段和附件元数据，交给后面的入库处理器；附件本体另走对象存储，本层只负责
如实上报 `filename`、`Content-Type` 和解码后的字节数。

请求体来自浏览器与脚本，完全不可信。

## 用法

```python
from formpost import parse_multipart

data = parse_multipart(content_type, body)

for field in data.fields:
    print(field.name, field.value)

for attachment in data.attachments:
    print(
        attachment.field_name,
        attachment.filename,
        attachment.content_type,
        attachment.size,
    )
```

`parse_multipart` 对可预期的坏请求抛出 `FormPostError` 的子类，见
`formpost/errors.py`；被拒绝的提交不会返回部分结果。

## 目录

```
formpost/
  __init__.py     对外导出
  parser.py       成帧与部件遍历
  headers.py      头部与参数解析、文件名选取
  body.py         传输解码
  limits.py       资源上限常量
  models.py       Field / Attachment / FormData
  errors.py       错误类型
docs/
  multipart-rules.md  行为规范（验收依据）
tests/
  test_parse.py
examples/
  demo.py         用一段样例请求跑通整条路径
```

## 开发

```bash
python -m pytest -q -p no:cacheprovider
python examples/demo.py
```

只依赖标准库；`pytest` 仅用于测试。

## 规范

`docs/multipart-rules.md` 是本层必须成立的性质清单——分界符识别、段头与
quoted-string、`filename*` 优先级、文件名中的控制字符、传输编码的严格性、
四类资源上限。改动本层之前请先读它。
