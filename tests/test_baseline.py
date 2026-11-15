from __future__ import annotations

import pytest

from pyscan import DEFAULT_RULE, analyze_source, analyze_paths


def scan(src: str) -> list:
    return analyze_source(src, "sample.py", DEFAULT_RULE)


def test_direct_sink_from_source():
    src = """
from flask import request
import os
os.system(request.args["cmd"])
"""
    findings = scan(src)
    assert len(findings) == 1
    assert findings[0].sink == "os.system"
    assert findings[0].rule == "cmd-injection"


def test_variable_propagation():
    src = """
import os
from flask import request
q = request.args["cmd"]
cmd = q
os.system(cmd)
"""
    assert len(scan(src)) == 1


def test_sanitizer_clears_taint():
    src = """
import os
import shlex
from flask import request
q = request.args["cmd"]
os.system("ls " + shlex.quote(q))
"""
    assert scan(src) == []


def test_clean_code_reports_nothing():
    src = """
import os
os.system("ls -l")
"""
    assert scan(src) == []


def test_sink_without_source_arg_is_ignored():
    # subprocess.run 的 arg=0 是命令数组；这里第一个实参干净，脏的在后头
    src = """
import subprocess
from flask import request
q = request.args["cmd"]
subprocess.run(["ls", q])
"""
    assert scan(src) == []


def test_syntax_error_raises():
    with pytest.raises(SyntaxError):
        scan("def broken(:\n    pass\n")


def test_analyze_paths_skips_non_python(tmp_path):
    (tmp_path / "a.py").write_text("print('hi')\n", encoding="utf-8")
    (tmp_path / "b.txt").write_text("import os\nos.system('x')\n", encoding="utf-8")
    assert analyze_paths([tmp_path / "a.py", tmp_path / "b.txt"], DEFAULT_RULE) == []


def test_analyze_paths_skips_syntax_error(tmp_path):
    bad = tmp_path / "bad.py"
    bad.write_text("def broken(:\n", encoding="utf-8")
    assert analyze_paths([bad], DEFAULT_RULE) == []


def test_analyze_paths_reads_utf8(tmp_path):
    good = tmp_path / "good.py"
    good.write_text(
        'import os\nfrom flask import request\nq = request.args["q"]\nos.system(q)\n',
        encoding="utf-8",
    )
    assert len(analyze_paths([good], DEFAULT_RULE)) == 1
