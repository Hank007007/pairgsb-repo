# tagwall

工单系统评论内容的入库前清洗层。前端用 `contenteditable` 编辑，提交 HTML
片段；服务端在写库前调用 `tagwall.sanitize()` 削掉危险标签与属性；展示页面把
存库结果直接赋给 `innerHTML`。

清洗口径见 [`docs/sanitize-policy.md`](docs/sanitize-policy.md)。

## 用法

```python
from tagwall import sanitize

clean = sanitize('<p onclick="x()">hello <a href="https://example.com">link</a></p>')
# '<p>hello <a href="https://example.com">link</a></p>'
```

## 结构

| 路径 | 作用 |
|---|---|
| `tagwall/sanitize.py` | 片段扫描与序列化入口 `sanitize()` |
| `tagwall/model.py` | 标签、属性、样式属性等静态策略表 |
| `tagwall/attrs.py` | 单个元素的属性过滤 |
| `tagwall/urls.py` | URL scheme 判定 |
| `tagwall/css.py` | `style` 属性声明过滤 |
| `tests/` | 富文本正常路径的回归测试 |

## 运行

```bash
python -m pytest -q -p no:cacheprovider
```

只依赖 Python 标准库与 pytest，没有运行时第三方依赖。
