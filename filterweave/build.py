"""过滤串拼装。"""

from .escape import escape_value

# 过滤串总长度上限（字符数）。
MAX_FILTER_LEN = 2048


def build_and(conditions, raw_filters=None):
    """拼装 AND 组合。

    ``conditions`` 是 ``(属性, 操作符, 值)`` 的序列；``raw_filters`` 用于接收
    调用方已经拼好的过滤片段。
    """
    parts = []
    for attr, op, value in conditions:
        parts.append(f"({attr}{op}{escape_value(value)})")
    if raw_filters:
        parts.extend(raw_filters)
    return "(&" + "".join(parts) + ")"


def build_or(conditions):
    """拼装 OR 组合。

    ``conditions`` 是 ``(属性, 操作符, 值)`` 的序列。
    """
    body = ""
    for attr, op, value in conditions:
        body += attr + op + escape_value(value)
    return "(|" + body + ")"


def build_present(attr, value="*"):
    """拼装存在性条件。

    ``value`` 的默认值 ``*`` 表示“该属性存在”。
    """
    return f"({attr}={value})"
