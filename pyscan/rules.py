"""内置规则集。真实项目里规则从 YAML 读，这里先写死一套够用的。"""

from __future__ import annotations

from .model import Rule, SanitizerSpec, SinkSpec

DEFAULT_RULE = Rule(
    id="cmd-injection",
    name="命令注入",
    sources=(
        "flask.request.args",
        "flask.request.form",
        "flask.request.values",
        "os.environ",
        "sys.argv",
        "input",
    ),
    sinks=(
        SinkSpec("os.system"),
        SinkSpec("os.popen"),
        SinkSpec("subprocess.run"),
        SinkSpec("subprocess.call"),
        SinkSpec("subprocess.Popen", arg=-1),
        SinkSpec("eval"),
        SinkSpec("exec"),
    ),
    sanitizers=(
        SanitizerSpec("shlex.quote"),
        SanitizerSpec("int"),
        SanitizerSpec("float"),
    ),
)
