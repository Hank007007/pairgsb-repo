"""filterweave —— 员工目录查询的过滤条件拼装件。"""

from .build import MAX_FILTER_LEN, build_and, build_or, build_present
from .errors import (
    FilterTooLongError,
    FilterWeaveError,
    InvalidFilterError,
    UnknownAttributeError,
)
from .escape import escape_dn_value, escape_value
from .schema import KNOWN, is_known_attr

__all__ = [
    "MAX_FILTER_LEN",
    "build_and",
    "build_or",
    "build_present",
    "escape_value",
    "escape_dn_value",
    "is_known_attr",
    "KNOWN",
    "FilterWeaveError",
    "UnknownAttributeError",
    "FilterTooLongError",
    "InvalidFilterError",
]
