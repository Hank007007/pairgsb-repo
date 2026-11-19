"""过滤值与 DN 值的转义。"""

_SPACE = " "

# §3.1：过滤值中必须转义的五个字符，统一两位小写十六进制。
_FILTER_ESCAPES = {
    "\\": "\\5c",
    "*": "\\2a",
    "(": "\\28",
    ")": "\\29",
    "\x00": "\\00",
}

# §8.1：DN 属性值中必须转义的字符。
_DN_ESCAPED_CHARS = frozenset(",+\"\\<>;=")


def escape_value(value):
    """转义过滤条件右侧的值。

    返回可以直接放进 ``attr=value`` 中 ``value`` 位置的文本。
    单遍扫描，来自输入的 ``\\`` 一律以 ``\\5c`` 出现，不会二次转义。
    """
    out = []
    for ch in value:
        out.append(_FILTER_ESCAPES.get(ch, ch))
    return "".join(out)


def escape_dn_value(value):
    """转义 DN 属性值。

    返回可以直接放进 ``attr=value`` 中 ``value`` 位置的文本。
    转义写法为反斜杠加字符本身（§8.4）。
    """
    if not value:
        return value
    start = 0
    end = len(value)
    while start < end and value[start] == _SPACE:
        start += 1
    while end > start and value[end - 1] == _SPACE:
        end -= 1
    out = ["\\ "] * start
    for i in range(start, end):
        ch = value[i]
        if ch in _DN_ESCAPED_CHARS:
            out.append("\\" + ch)
        elif ch == "#" and i == 0:
            out.append("\\#")
        else:
            out.append(ch)
    out.extend(["\\ "] * (len(value) - end))
    return "".join(out)
