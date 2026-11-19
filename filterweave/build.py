"""过滤串拼装。"""

from .errors import FilterTooLongError, InvalidFilterError
from .escape import escape_value
from .schema import require_known

# 过滤串总长度上限（字符数）。
MAX_FILTER_LEN = 2048


def _check_length(result):
    if len(result) > MAX_FILTER_LEN:
        raise FilterTooLongError(len(result), MAX_FILTER_LEN)
    return result


def _build_group(operator, conditions):
    parts = []
    for attr, op, value in conditions:
        require_known(attr)
        parts.append(f"({attr}{op}{escape_value(value)})")
    return _check_length("(" + operator + "".join(parts) + ")")


def build_and(conditions):
    """拼装 AND 组合。

    ``conditions`` 是 ``(属性, 操作符, 值)`` 的序列；每个条件各自成组。
    空集合返回恒真过滤串 ``(&)``（§5.4）。
    """
    return _build_group("&", conditions)


def build_or(conditions):
    """拼装 OR 组合。

    ``conditions`` 是 ``(属性, 操作符, 值)`` 的序列；每个条件各自成组。
    空集合返回恒假过滤串 ``(|)``（§5.4）。
    """
    return _build_group("|", conditions)


def build_present(attr, value="*"):
    """拼装存在性条件，输出 ``(attr=*)``。

    ``*`` 是协议语法的一部分，不是值；按 §4.4，传入其它值一律拒绝。
    """
    require_known(attr)
    if value != "*":
        raise InvalidFilterError(
            "present condition does not accept a custom value"
        )
    return _check_length(f"({attr}=*)")
