"""异常类型。"""


class FilterWeaveError(Exception):
    """拼装过滤条件时的输入错误基类。"""


class UnknownAttributeError(FilterWeaveError):
    """属性名不在属性表中。"""

    def __init__(self, attribute):
        self.attribute = attribute
        super().__init__(f"unknown attribute: {attribute}")


class FilterTooLongError(FilterWeaveError):
    """过滤串超长度上限。"""

    def __init__(self, length, limit):
        self.length = length
        self.limit = limit
        super().__init__(
            f"filter too long: {length} characters (limit {limit})"
        )


class InvalidFilterError(FilterWeaveError):
    """条件本身不合法。"""
