# pyscan 规则语义

`pyscan` 目前只跑一条规则（命令注入），但引擎本身是通用的：来源、危险点、净化器都由
`Rule` 描述。这份文档定的是**引擎该有的语义**，不是某条规则的配置。实现跟它对不上
就是引擎的 bug，改引擎，别改规则。

## 1. 分析方式

- 纯 AST，不 import 被扫代码、不执行任何东西。
- 单个文件独立分析。跨文件、跨模块的追踪**不在本期范围**（明确不做）。
- 一个文件里能拿到的信息就这些：赋值、调用、函数定义与调用、属性访问、下标。

## 2. 名字怎么匹配

规则里的条目是**点分全名**：`os.system`、`flask.request.args`。

### 2.1 先把别名抹平

匹配之前必须把 `import` 造成的别名还原成全名：

```python
import os as o                 # o.system        -> os.system
from os import system          # system          -> os.system
from flask import request      # request.args    -> flask.request.args
import subprocess as sp        # sp.run          -> subprocess.run
```

只跟被扫文件自己的 `import` 语句走，不做包解析。

### 2.2 匹配口径

归一化之后的点分全名，与条目按**组件边界**比较，两种情况都算命中：

- 相等：`os.system` 命中 `os.system`
- 条目的下级：`os.environ.get` 命中 `os.environ`

**不**做后缀匹配：`request.args` 不等于 `flask.request.args`，
必须靠 §2.1 的别名还原对上，不能靠"尾巴像"。

## 3. 污点怎么传

### 3.1 赋值

下面这些赋值目标都算传播，值脏就整条记脏：

```python
cmd = q                      # Name
self.cmd = q                 # Attribute
opts["cmd"] = q              # Subscript
a, b = q, "safe"             # Tuple（逐元素）
```

一条语句里只要右侧脏，左侧所有目标都脏。链式赋值 `a = b = q` 同样。

### 3.2 函数

同一个文件内，**一层**直接调用要传：

```python
def run(cmd):        # 形参 cmd 接下实参的污点
    os.system(cmd)   # 命中

run(user_input)
```

```python
def get_q():
    return request.args["q"]     # 返回值带污点

os.system(get_q())               # 命中
```

递归、多层调用链、方解析（`self.get()` 找到 `get`）都不要求。

### 3.3 组合表达式

只要子表达式脏，整个表达式就脏：

```python
os.system("ls " + q)             # BinOp
os.system(f"ls {q}")             # JoinedStr
os.system("ls %s" % q)           # BinOp(%)
os.system("".join([q]))          # Call，实参脏即结果脏
os.system(q if c else "ls")      # IfExp
os.system([q][0])                # Subscript
os.system({"k": q}["k"])         # Dict
```

`Call` 的返回值：净化器（见 §3.4）之外，**实参脏则返回值脏**。

### 3.4 净化器

`SanitizerSpec.name` 按 §2.1、§2.2 归一化后精确匹配，**不看名字尾巴**。
`arg` 指出被净化的实参下标（`-1` 表示都算）。语义是：

- 净化器调用本身的结果**不脏**；
- 净化器**别的实参**保持原样，该脏还是脏。

```python
os.system(shlex.quote(q))            # 干净，不报
os.system("ls " + shlex.quote(q))    # 干净，不报
os.system(shlex.quote("ls") + q)     # 报，q 没被净化
```

### 3.5 作用域

- 进函数就清空局部变量表，函数内的赋值不外溢到模块级。
- 函数里的局部变量脏了，就在这个函数里报。
- **同一个类的所有方法共用一个 `self.*` 属性表**：`__init__` 里写进 `self.cmd` 的污点，
  别的实例方法里读 `self.cmd` 要接着脏。类的其它属性按模块级变量处理。

## 4. 报出来的位置

`Finding.line` / `Finding.col` 是**危险调用那一行/那一列**，不是污染源的位置。
`trace` 保留来源与危险点两段，够人顺藤摸瓜就行。

## 5. 报告口径

- 同一个文件里同一个危险调用只报一条。
- 语法错误的文件直接跳过，不中断整轮扫描（这是 `analyze_paths` 的约定，
  调用方靠返回的条数变化感知）。
