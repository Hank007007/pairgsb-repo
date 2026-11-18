"""过滤值与 DN 值的转义。"""

_SPACE = " "


def escape_value(value):
    """转义过滤条件右侧的值。

    返回可以直接放进 ``attr=value`` 中 ``value`` 位置的文本。
    """
    out = []
    for ch in value:
        if ch == "*":
            out.append("\\2a")
        elif ch == "(":
            out.append("\\28")
        else:
            out.append(ch)
    return "".join(out)


def escape_dn_value(value):
    """转义 DN 属性值。

    返回可以直接放进 ``attr=value`` 中 ``value`` 位置的文本。
    """
    if not value:
        return value
    start = 0
    end = len(value)
    while start < end and value[start] == _SPACE:
        start += 1
    while end > start and value[end - 1] == _SPACE:
        end -= 1
    if start == 0 and end == len(value):
        return value
    return "\\ " * start + value[start:end] + "\\ " * (len(value) - end)
