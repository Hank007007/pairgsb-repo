# tplsentry

`tplsentry` 是运营平台的通知模板引擎。运营在后台编写模板，系统把告警、账单
等数据填进去，渲染成发给用户的邮件。

```jinja
Hello {{ user.name }},

{% for item in items %}
- {{ item.title }} ({{ item.count }})
{% endfor %}

{% include "partials/footer.html" %}
```

模板与数据都按不可信输入对待，边界条款见
[`docs/sandbox-rules.md`](docs/sandbox-rules.md)，结构说明见
[`docs/design.md`](docs/design.md)。

## 使用

```python
from tplsentry import TemplateLoader, render

loader = TemplateLoader("templates/")
template = loader.get_template("notice.html")
html = render(template, {"user": {"name": "Alice"}, "items": items})
```

也可以直接从字符串构造模板：

```python
from tplsentry import Template, render

render(Template("<p>{{ nickname }}</p>"), {"nickname": "<b>Alice</b>"})
# '<p>&lt;b&gt;Alice&lt;/b&gt;</p>'
```

## 资源上限

```python
from tplsentry import Limits, Template, render

limits = Limits(max_loop_iterations=500, max_output_bytes=32_768, max_render_seconds=1.0)
render(Template(source), context, limits=limits)
```

## 开发

只依赖标准库与 pytest：

```
python -m pytest -q
```

目录结构：

```
tplsentry/            引擎实现
tests/                单元测试
tests/fixtures/       测试用模板目录
docs/                 边界条款与设计说明
```
