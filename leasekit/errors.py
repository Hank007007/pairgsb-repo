"""leasekit 的异常层次。"""

from __future__ import annotations


class LeaseError(Exception):
    """leasekit 所有异常的基类。"""


class LeaseNotHeld(LeaseError):
    """当前调用方（或某个第三方）并未持有目标资源的租约。

    参数：
        resource: 资源名。
        holder:   当前实际持有者的 owner；无人持有时为空字符串。
    """

    def __init__(self, resource: str, holder: str = "") -> None:
        self.resource = resource
        self.holder = holder
        if holder:
            message = f"resource {resource!r} is held by {holder!r}"
        else:
            message = f"resource {resource!r} is not held"
        super().__init__(message)


class LeaseExpired(LeaseError):
    """租约已过期，不能再 renew。

    参数：
        resource: 资源名。
    """

    def __init__(self, resource: str) -> None:
        self.resource = resource
        super().__init__(f"lease on resource {resource!r} has expired")
