"""租约服务使用的异常类型。

所有异常都继承 :class:`LeaseError`，调用方可以只捕获基类。
"""

from __future__ import annotations


class LeaseError(Exception):
    """租约相关错误的基类。"""


class InvalidLease(LeaseError, ValueError):
    """租约字段不合法。"""


class DuplicateLease(InvalidLease):
    """租约 ID 已存在。"""


class UnknownLease(LeaseError, KeyError):
    """租约 ID 不存在。"""


class LeaseNotActive(LeaseError):
    """租约已吊销或已过期，不能再参与派生操作。"""


class InvalidScope(LeaseError, ValueError):
    """scope 文本不合法。"""


class InvalidTTL(LeaseError, ValueError):
    """TTL 不合法（非正数、类型错误或超过全局上限）。"""


class TTLExceedsPolicy(InvalidTTL):
    """请求的 TTL 超 policy 为该 scope 设定的上限。"""


class UnknownScope(LeaseError, LookupError):
    """policy 中找不到适用于该 scope 的规则，且没有兜底规则。"""
