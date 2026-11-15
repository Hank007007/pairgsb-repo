# 扫描器这周的反馈

> 从内部群里摘的，按时间倒序。都是"扫不出来"这一类。

## 2026-11-13 · 应用安全组：三条典型样本一个都没报

我们拿了三个真实漏洞的代码片段喂进去，`pyscan` 一个都没报：

```python
# 1
import os as o
from flask import request
o.system(request.args["cmd"])

# 2
def build():
    return request.args["cmd"]

os.system("ls " + build())

# 3
class Runner:
    def __init__(self, q):
        self.cmd = q

    def go(self):
        os.system(self.cmd)
```

人工看都是明显的命令注入。第 2 段里就算把 `build()` 换成变量也一样，
只要中间拼过字符串就丢。

## 2026-11-12 · 平台组：报了但行号不对

扫一个 800 行的文件，命中的行号指在 `q = request.args["cmd"]` 那行，
真正的 `os.system(cmd)` 在 40 行以后。审代码的人得自己往下找，
等于这条结果只告诉我们"这个文件里有"，没法直接定位。

## 2026-11-10 · 应用安全组：包了一层的函数就不报了

```python
def run(cmd):
    os.system(cmd)

run(request.args["cmd"])
```

这种把危险调用包进小函数的写法在我们代码里到处都是，现在全漏。
