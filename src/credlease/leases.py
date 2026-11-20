"""Lease records and the in-process lease state machine."""

from __future__ import annotations

import copy
from dataclasses import dataclass
from datetime import datetime, timedelta
from enum import Enum
from itertools import count
from typing import TYPE_CHECKING, Iterable

from .backend import Credential, CredentialBackend, Revoke
from .clock import Clock

if TYPE_CHECKING:  # pragma: no cover - typing only
    from .cache import LeaseCache


class LeaseError(Exception):
    """Base error for lease operations."""


class UnknownLeaseError(LeaseError):
    """Raised when a lease id is not known to the store."""


class LeaseStateError(LeaseError):
    """Raised when an operation is not allowed in the current state."""


class LeaseExpiredError(LeaseStateError):
    """Raised when an operation targets a lease that is no longer live."""


class LeaseState(str, Enum):
    ISSUED = "issued"
    ACTIVE = "active"
    RENEWED = "renewed"
    REVOKED = "revoked"
    EXPIRED = "expired"

    @property
    def is_live(self) -> bool:
        return self in (LeaseState.ISSUED, LeaseState.ACTIVE, LeaseState.RENEWED)

    @property
    def is_terminal(self) -> bool:
        return not self.is_live


@dataclass
class Lease:
    """One issued credential together with its lifecycle metadata."""

    lease_id: str
    subject: str
    role: str
    credential: Credential
    issued_at: datetime
    expires_at: datetime
    absolute_expires_at: datetime
    soft_ttl: int
    max_ttl: int
    version: int
    state: LeaseState = LeaseState.ISSUED
    renewed_at: datetime | None = None
    revoked_at: datetime | None = None
    expired_at: datetime | None = None
    renewals: int = 0

    @property
    def is_live(self) -> bool:
        return self.state.is_live

    @property
    def ttl_seconds(self) -> float:
        return (self.expires_at - self.issued_at).total_seconds()

    def is_expired(self, now: datetime) -> bool:
        """Whether the lease is unusable at ``now`` because of time or state."""

        if self.state is LeaseState.EXPIRED:
            return True
        return now >= self.expires_at or now >= self.absolute_expires_at

    def snapshot(self) -> "Lease":
        return copy.deepcopy(self)


class LeaseStore:
    """Authoritative in-memory registry of leases."""

    def __init__(
        self,
        backend: CredentialBackend,
        clock: Clock,
        cache: "LeaseCache | None" = None,
    ) -> None:
        self._backend = backend
        self._clock = clock
        self._cache = cache
        self._leases: dict[str, Lease] = {}
        self._ids = count(1)

    def create(
        self,
        subject: str,
        role: str,
        credential: Credential,
        ttl_seconds: int,
        *,
        soft_ttl: int,
        max_ttl: int,
        version: int,
    ) -> Lease:
        now = self._clock.now()
        lease = Lease(
            lease_id=f"lease-{next(self._ids):06d}",
            subject=subject,
            role=role,
            credential=credential,
            issued_at=now,
            expires_at=now + timedelta(seconds=ttl_seconds),
            absolute_expires_at=now + timedelta(seconds=max_ttl),
            soft_ttl=soft_ttl,
            max_ttl=max_ttl,
            version=version,
        )
        self._leases[lease.lease_id] = lease
        return lease.snapshot()

    def get(self, lease_id: str) -> Lease:
        try:
            return self._leases[lease_id]
        except KeyError as exc:
            raise UnknownLeaseError(f"unknown lease: {lease_id!r}") from exc

    def refresh(self, lease_id: str) -> Lease:
        """Return the lease, settling it to ``expired`` when time has run out."""

        lease = self.get(lease_id)
        if lease.is_live and lease.is_expired(self._clock.now()):
            self.expire(lease_id)
        return lease

    def mark_active(self, lease_id: str) -> Lease:
        lease = self.get(lease_id)
        if lease.state is LeaseState.ISSUED:
            lease.state = LeaseState.ACTIVE
        return lease

    def renew(self, lease_id: str, expires_at: datetime) -> Lease:
        lease = self.get(lease_id)
        if not lease.is_live:
            return lease
        now = self._clock.now()
        if now >= lease.absolute_expires_at:
            self.expire(lease_id)
            raise LeaseExpiredError(
                f"lease {lease_id!r} has reached its absolute expiry"
            )
        if expires_at > lease.absolute_expires_at:
            expires_at = lease.absolute_expires_at
        lease.state = LeaseState.RENEWED
        lease.expires_at = expires_at
        lease.renewed_at = now
        lease.renewals += 1
        if self._cache is not None:
            self._cache.put(
                self._cache.key(lease.subject, lease.role, lease.version),
                lease.snapshot(),
            )
        return lease

    def expire(self, lease_id: str) -> Lease:
        lease = self.get(lease_id)
        if lease.is_live:
            lease.state = LeaseState.EXPIRED
            lease.expired_at = self._clock.now()
            if self._cache is not None:
                self._cache.drop(
                    self._cache.key(lease.subject, lease.role, lease.version)
                )
        return lease

    def revoke(self, subject: str, role: str) -> list[Lease]:
        """Revoke every live lease the subject holds for ``role``.

        Returns the leases affected by the call.
        """

        live = [
            lease
            for lease in self._leases.values()
            if lease.subject == subject and lease.role == role and lease.is_live
        ]
        if not live:
            return []

        now = self._clock.now()
        revoked: list[Lease] = []
        for lease in sorted(live, key=lambda item: (item.issued_at, item.lease_id)):
            Revoke(self._backend, lease.credential.credential_id)
            lease.state = LeaseState.REVOKED
            lease.revoked_at = now
            if self._cache is not None:
                self._cache.drop(
                    self._cache.key(lease.subject, lease.role, lease.version)
                )
            revoked.append(lease.snapshot())
        return revoked

    def live_leases(self, subject: str | None = None, role: str | None = None) -> list[Lease]:
        return [
            lease.snapshot()
            for lease in self._leases.values()
            if lease.is_live
            and (subject is None or lease.subject == subject)
            and (role is None or lease.role == role)
        ]

    def all_leases(self) -> list[Lease]:
        return [lease.snapshot() for lease in self._leases.values()]

    def __len__(self) -> int:
        return len(self._leases)

    def __iter__(self) -> Iterable[Lease]:
        return iter(self._leases.values())
