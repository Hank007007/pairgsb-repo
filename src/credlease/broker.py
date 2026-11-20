"""Public broker API: request, renew and revoke credential leases."""

from __future__ import annotations

from datetime import timedelta

from .backend import BackendError, CredentialBackend
from .cache import LeaseCache
from .clock import Clock
from .leases import (
    Lease,
    LeaseExpiredError,
    LeaseStore,
)
from .policy import PolicyStore, resolve_ttl


class BrokerError(Exception):
    """Base class for broker-level failures."""


class Broker:
    """Issues, renews and revokes leases for configured roles."""

    def __init__(
        self,
        policies: PolicyStore,
        backend: CredentialBackend,
        clock: Clock,
        cache: LeaseCache | None = None,
    ) -> None:
        self._policies = policies
        self._backend = backend
        self._clock = clock
        self._cache = cache if cache is not None else LeaseCache()
        self._store = LeaseStore(backend, clock, self._cache)

    @property
    def cache(self) -> LeaseCache:
        return self._cache

    @property
    def store(self) -> LeaseStore:
        return self._store

    def request_lease(
        self,
        subject: str,
        role: str,
        *,
        ttl: float | None = None,
        ttl_seconds: float | None = None,
    ) -> Lease:
        """Obtain a lease for ``subject`` acting in ``role``.

        A live, policy-compatible cache entry is reused when the role allows
        it; otherwise a fresh credential is requested from the backend.
        """

        policy = self._policies.get(role)
        key = self._cache.key(subject, role, policy.version)

        if policy.reuse:
            cached = self._cache.get(key)
            if cached is not None:
                current = self._store.refresh(cached.lease_id)
                if current.is_live:
                    self._store.mark_active(current.lease_id)
                    active = current.snapshot()
                    self._cache.put(key, active)
                    return active
                self._cache.drop(key)

        seconds = resolve_ttl(policy, ttl=ttl, ttl_seconds=ttl_seconds)
        try:
            credential = self._backend.issue(subject, role, policy.backend, seconds)
        except BackendError as exc:
            raise BrokerError(f"backend could not issue a credential: {exc}") from exc

        lease = self._store.create(
            subject,
            role,
            credential,
            seconds,
            soft_ttl=policy.soft_ttl,
            max_ttl=policy.max_ttl,
            version=policy.version,
        )
        active = self._store.mark_active(lease.lease_id).snapshot()
        self._cache.put(key, active)
        return active

    def renew_lease(self, lease_id: str) -> Lease:
        """Extend a live lease's soft expiry from the current time."""

        lease = self._store.get(lease_id)
        if not lease.is_live:
            return lease.snapshot()
        now = self._clock.now()
        if now >= lease.absolute_expires_at:
            self._store.expire(lease_id)
            raise LeaseExpiredError(
                f"lease {lease_id!r} has reached its absolute expiry"
            )
        if now >= lease.expires_at:
            self._store.expire(lease_id)
            raise LeaseExpiredError(f"lease {lease_id!r} has expired")

        policy = self._policies.get(lease.role)
        target = now + timedelta(seconds=policy.soft_ttl)
        if target > lease.absolute_expires_at:
            target = lease.absolute_expires_at
        renewed = self._store.renew(lease_id, target)
        return renewed.snapshot()

    def revoke(self, subject: str, role: str) -> list[Lease]:
        """Revoke ``subject``'s leases for ``role``.

        Returns the leases that were revoked by this call.
        """

        self._policies.get(role)
        return self._store.revoke(subject, role)

    def get_lease(self, lease_id: str) -> Lease:
        return self._store.refresh(lease_id).snapshot()
