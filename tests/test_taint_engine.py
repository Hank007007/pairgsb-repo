"""按 docs/rule-semantics.md 补的引擎语义用例。

每条 docs/feedback-notes.md 的反馈都有对应测试，组合表达式覆盖 §3.3 全部写法。
基线在 tests/test_baseline.py，这里不动它。
"""

from __future__ import annotations

from pyscan import DEFAULT_RULE, analyze_source


def scan(src: str):
    return analyze_source(src, "sample.py", DEFAULT_RULE)


# ---- 反馈 2026-11-13：三条真实样本 --------------------------------


def test_feedback_2026_11_13_sample_1_import_alias():
    src = (
        "import os as o\n"
        "from flask import request\n"
        "o.system(request.args['cmd'])\n"
    )
    findings = scan(src)
    assert len(findings) == 1
    assert findings[0].sink == "os.system"


def test_feedback_2026_11_13_sample_2_return_through_function_and_concat():
    src = (
        "import os\n"
        "from flask import request\n"
        "def build():\n"
        "    return request.args['cmd']\n"
        "os.system('ls ' + build())\n"
    )
    findings = scan(src)
    assert len(findings) == 1
    assert findings[0].line == 5


def test_feedback_2026_11_13_sample_3_self_attribute_across_methods():
    src = (
        "from flask import request\n"
        "import os\n"
        "class Runner:\n"
        "    def __init__(self, q):\n"
        "        self.cmd = q\n"
        "    def go(self):\n"
        "        os.system(self.cmd)\n"
        "Runner(request.args['cmd']).go()\n"
    )
    findings = scan(src)
    assert len(findings) == 1
    assert findings[0].line == 7
    assert findings[0].sink == "os.system"


# ---- 反馈 2026-11-12：行号落在危险调用处 ---------------------------


def test_feedback_2026_11_12_line_points_at_sink_not_source():
    src = (
        "import os\n"
        "from flask import request\n"
        "q = request.args['cmd']\n"
        "x = 1\n"
        "y = 2\n"
        "os.system(q)\n"
    )
    findings = scan(src)
    assert len(findings) == 1
    assert findings[0].line == 6
    assert findings[0].col == 0


# ---- 反馈 2026-11-10：包一层的函数要传污点 --------------------------


def test_feedback_2026_11_10_taint_passes_through_one_function_layer():
    src = (
        "import os\n"
        "from flask import request\n"
        "def run(cmd):\n"
        "    os.system(cmd)\n"
        "run(request.args['cmd'])\n"
    )
    findings = scan(src)
    assert len(findings) == 1
    assert findings[0].line == 4


# ---- §2.1 别名还原 -------------------------------------------------


def test_from_import_function_alias_is_normalized():
    src = (
        "from os import system\n"
        "from flask import request\n"
        "system(request.args['cmd'])\n"
    )
    assert len(scan(src)) == 1


def test_module_alias_attribute_chain_is_normalized():
    src = (
        "import subprocess as sp\n"
        "from flask import request\n"
        "sp.run(request.args['cmd'])\n"
    )
    assert len(scan(src)) == 1


def test_no_suffix_matching_without_alias():
    # request.args 不等于 flask.request.args，没 import 别名就不能靠后缀命中
    src = (
        "import os\n"
        "request = object()\n"
        "os.system(request.args['cmd'])\n"
    )
    assert scan(src) == []


# ---- §3.1 赋值目标：Attribute / Subscript / Tuple / 链式 -------------


def test_attribute_assignment_propagates():
    src = (
        "import os\n"
        "from flask import request\n"
        "cfg = object()\n"
        "cfg.cmd = request.args['cmd']\n"
        "os.system(cfg.cmd)\n"
    )
    assert len(scan(src)) == 1


def test_subscript_assignment_propagates():
    src = (
        "import os\n"
        "from flask import request\n"
        "opts = {}\n"
        "opts['cmd'] = request.args['cmd']\n"
        "os.system(opts['cmd'])\n"
    )
    assert len(scan(src)) == 1


def test_tuple_assignment_propagates_per_element():
    src = (
        "import os\n"
        "from flask import request\n"
        "a, b = request.args['cmd'], 'safe'\n"
        "os.system(a)\n"
    )
    assert len(scan(src)) == 1


def test_chain_assignment_propagates_to_every_target():
    src = (
        "import os\n"
        "from flask import request\n"
        "a = b = request.args['cmd']\n"
        "os.system(b)\n"
    )
    assert len(scan(src)) == 1


# ---- §3.2 一层函数：形参与返回值 ------------------------------------


def test_function_return_value_carries_taint_into_concat():
    src = (
        "import os\n"
        "from flask import request\n"
        "def get_q():\n"
        "    return request.args['q']\n"
        "os.system(get_q())\n"
    )
    findings = scan(src)
    assert len(findings) == 1
    assert findings[0].line == 5


def test_clean_argument_keeps_wrapper_function_clean():
    src = (
        "import os\n"
        "def run(cmd):\n"
        "    os.system(cmd)\n"
        "run('ls -l')\n"
    )
    assert scan(src) == []


def test_multi_level_call_chain_is_not_in_scope():
    # 多层调用链明确不做：第二层的 run 不被追踪
    src = (
        "import os\n"
        "from flask import request\n"
        "def run(cmd):\n"
        "    os.system(cmd)\n"
        "def middle(x):\n"
        "    run(x)\n"
        "middle(request.args['cmd'])\n"
    )
    assert scan(src) == []


def test_recursive_function_does_not_blow_up():
    # 递归不在范围，但不能无限展开；最外层那一层调用仍要分析
    src = (
        "import os\n"
        "from flask import request\n"
        "def walk(n):\n"
        "    if n:\n"
        "        return walk(n - 1)\n"
        "    os.system(n)\n"
        "walk(request.args['x'])\n"
    )
    findings = scan(src)
    assert len(findings) == 1
    assert findings[0].line == 6


def test_keyword_argument_binds_to_parameter():
    src = (
        "import os\n"
        "from flask import request\n"
        "def run(cmd):\n"
        "    os.system(cmd)\n"
        "run(cmd=request.args['x'])\n"
    )
    findings = scan(src)
    assert len(findings) == 1
    assert findings[0].line == 4


def test_self_method_resolution_is_not_done():
    # 方解析（self.get() 找到 get）明确不做
    src = (
        "from flask import request\n"
        "import os\n"
        "class Reader:\n"
        "    def get(self):\n"
        "        return request.args['x']\n"
        "    def run(self):\n"
        "        os.system(self.get())\n"
        "Reader().run()\n"
    )
    assert scan(src) == []


# ---- §3.3 组合表达式：文档列出的全部写法 -----------------------------


COMBINATIONS = {
    "binop_add": "os.system('ls ' + q)",
    "joinedstr": "os.system(f'ls {q}')",
    "binop_percent": "os.system('ls %s' % q)",
    "join_call": "os.system(''.join([q]))",
    "ifexp": "os.system(q if c else 'ls')",
    "subscript": "os.system([q][0])",
    "dict": "os.system({'k': q}['k'])",
}


def _combination_src(expression: str) -> str:
    return (
        "import os\n"
        "from flask import request\n"
        "q = request.args['q']\n"
        "c = True\n"
        f"{expression}\n"
    )


def test_all_documented_combinations_are_tainted():
    for name, expression in COMBINATIONS.items():
        findings = scan(_combination_src(expression))
        assert len(findings) == 1, f"{name} 漏报"
        assert findings[0].line == 5, f"{name} 行号不对"


def test_non_sanitizer_call_result_is_tainted_via_argument():
    # 净化器之外的 Call：实参脏则返回值脏
    src = (
        "import os\n"
        "from flask import request\n"
        "def wrap(x):\n"
        "    return x\n"
        "q = request.args['q']\n"
        "os.system(wrap(q))\n"
    )
    assert len(scan(src)) == 1


# ---- §3.4 净化器：全名精确匹配 --------------------------------------


def test_sanitizer_result_is_clean():
    src = (
        "import os\n"
        "import shlex\n"
        "from flask import request\n"
        "q = request.args['cmd']\n"
        "os.system(shlex.quote(q))\n"
    )
    assert scan(src) == []


def test_sanitizer_inside_concat_is_clean():
    src = (
        "import os\n"
        "import shlex\n"
        "from flask import request\n"
        "q = request.args['cmd']\n"
        "os.system('ls ' + shlex.quote(q))\n"
    )
    assert scan(src) == []


def test_uncovered_tainted_sibling_argument_still_reports():
    src = (
        "import os\n"
        "import shlex\n"
        "from flask import request\n"
        "q = request.args['cmd']\n"
        "os.system(shlex.quote('ls') + q)\n"
    )
    findings = scan(src)
    assert len(findings) == 1
    assert findings[0].line == 5


def test_sanitizer_matches_full_name_not_tail():
    # 名字尾巴像 quote 但全名不是 shlex.quote，不能当净化器
    src = (
        "import os\n"
        "from flask import request\n"
        "class Fake:\n"
        "    @staticmethod\n"
        "    def quote(x):\n"
        "        return x\n"
        "shlex = Fake()\n"
        "q = request.args['cmd']\n"
        "os.system(shlex.quote(q))\n"
    )
    findings = scan(src)
    assert len(findings) == 1


def test_builtin_int_sanitizer_clears_taint():
    src = (
        "import os\n"
        "from flask import request\n"
        "q = request.args['cmd']\n"
        "os.system(str(int(q)))\n"
    )
    assert scan(src) == []


# ---- §3.5 作用域 ----------------------------------------------------


def test_function_local_assignment_does_not_escape():
    src = (
        "import os\n"
        "from flask import request\n"
        "def take():\n"
        "    local_q = request.args['cmd']\n"
        "take()\n"
        "os.system(local_q)\n"
    )
    assert scan(src) == []


def test_self_attribute_taint_shared_between_methods():
    src = (
        "from flask import request\n"
        "import os\n"
        "class Runner:\n"
        "    def __init__(self, q):\n"
        "        self.cmd = q\n"
        "    def go(self):\n"
        "        cmd = self.cmd\n"
        "        os.system(cmd)\n"
    )
    findings = scan(src)
    assert len(findings) == 1
    assert findings[0].line == 8


def test_method_parameter_not_stored_on_self_is_not_tainted():
    # 没存进 self.* 的形参不能凭空变脏：__init__ 的 self 属性语义只限 self 赋值
    src = (
        "import os\n"
        "class Runner:\n"
        "    def go(self, cmd):\n"
        "        os.system(cmd)\n"
        "Runner().go('ls -l')\n"
    )
    assert scan(src) == []


def test_only_self_stored_parameter_is_seeded_tainted():
    # __init__ 里只有写进 self 的形参算脏，普通局部形参不连带变脏
    src = (
        "import os\n"
        "class Runner:\n"
        "    def __init__(self, q, note):\n"
        "        self.cmd = q\n"
        "        self.label = note\n"
        "    def go(self):\n"
        "        os.system(self.cmd)\n"
        "    def echo(self):\n"
        "        print(self.label)\n"
    )
    findings = scan(src)
    assert len(findings) == 1
    assert findings[0].line == 7


# ---- §4/§5 报告口径 -------------------------------------------------


def test_trace_keeps_source_and_sink_segments():
    src = (
        "import os\n"
        "from flask import request\n"
        "os.system(request.args['cmd'])\n"
    )
    findings = scan(src)
    assert len(findings) == 1
    assert findings[0].trace == ("flask.request.args", "os.system")


def test_same_sink_call_reported_only_once():
    src = (
        "import os\n"
        "from flask import request\n"
        "q = request.args['cmd']\n"
        "r = request.args['cmd']\n"
        "os.system(q + r)\n"
    )
    findings = scan(src)
    assert len(findings) == 1


def test_bare_collection_literal_as_sink_arg_is_not_tainted():
    # subprocess.run arg=0 是命令数组：容器里的元素脏不等于数组实参脏
    src = (
        "import subprocess\n"
        "from flask import request\n"
        "q = request.args['cmd']\n"
        "subprocess.run(['ls', q])\n"
    )
    assert scan(src) == []
