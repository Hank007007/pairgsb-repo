"""租约表。

租约是本进程内保存的短期凭据记录；:class:`LeaseStore` 负责保存、查询、
活跃判断与到期回收。所有时间判断都通过注入的 :class:`~leaseledger.clock.Clock`
读取当前时刻，便于测试推进时间。
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime

from .clock import Clock
from .errors import DuplicateLease, UnknownLease


@dataclass
class Lease:
    """一条已签发的租约。"""

    id: str
    subject: str
    scopes: tuple[str, ...]
    issued_at: datetime
    expires_at: datetime
    parent_lease_id: str | None = None
    revoked_at: datetime | None = None
    revoked_reason: str | None = None

    @property
    def ttl_seconds(self) -> int:
        return int((self.expires_at - self.issued_at).total_seconds())

    @property
    def revoked(self) -> bool:
        return self.revoked_at is not None


class LeaseStore:
    """进程内租约表。"""

    def __init__(self, clock: Clock) -> None:
        self._clock = clock
        self._leases: dict[str, Lease] = {}

    def __len__(self) -> int:
        return len(self._leases)

    def __contains__(self, lease_id: object) -> bool:
        return lease_id in self._leases

    def put(self, lease: Lease) -> Lease:
        """写入一条新租约。"""

        if lease.id in self._leases:
            raise DuplicateLease(f"租约已存在: {lease.id!r}")
        self._leases[lease.id] = lease
        return lease

    def get(self, lease_id: str) -> Lease:
        try:
            return self._leases[lease_id]
        except KeyError:
            raise UnknownLease(lease_id) from None

    def all_leases(self) -> tuple[Lease, ...]:
        return tuple(self._leases.values())

    def children_of(self, lease_id: str) -> tuple[Lease, ...]:
        """直接以该租约父租约的所有租约。"""

        return tuple(
            lease
            for lease in self._leases.values()
            if lease.parent_lease_id == lease_id
        )

    def mark_revoked(
        self, lease_id: str, *, at: datetime, reason: str | None = None
    ) -> Lease:
        lease = self.get(lease_id)
        if lease.revoked_at is None:
            lease.revoked_at = at
            lease.revoked_reason = reason
        return lease

    def is_active(self, lease: Lease) -> bool:
        """租约当前是否可用于鉴权。"""

        if lease.revoked_at is not None:
            return False
        return lease.expires_at >= self._clock.now()

    def active_leases(self) -> tuple[Lease, ...]:
        return tuple(lease for lease in self._leases.values() if self.is_active(lease))

    def purge_expired(self) -> tuple[str, ...]:
        """回收已经过期且未被吊销的租约，返回被移除的 ID。"""

        doomed = [
            lease.id
            for lease in self._leases.values()
            if lease.revoked_at is None and not self.is_active(lease)
        ]
        for lease_id in doomed:
            del self._leases[lease_id]
        return tuple(doomed)
