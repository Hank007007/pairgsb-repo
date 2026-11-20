"""Credential backend protocol and an in-memory implementation."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from itertools import count
from typing import Protocol, runtime_checkable


class BackendError(Exception):
    """Raised when the credential backend cannot complete an operation."""


@dataclass(frozen=True)
class Credential:
    """A credential handed out by the backend for one lease."""

    credential_id: str
    subject: str
    role: str
    backend: str
    username: str
    password: str
    issued_at: datetime


@runtime_checkable
class CredentialBackend(Protocol):
    """Minimal contract every credential source must satisfy."""

    def issue(
        self, subject: str, role: str, backend: str, ttl_seconds: int
    ) -> Credential:
        """Create a credential valid for ``ttl_seconds``."""

    def revoke(self, credential_id: str) -> bool:
        """Invalidate a credential. Return ``True`` when it is gone."""


class InMemoryBackend:
    """In-process backend useful for development and tests."""

    def __init__(self) -> None:
        self._ids = count(1)
        self._credentials: dict[str, Credential] = {}
        self._revoked: set[str] = set()
        self.issue_count = 0
        self.revoke_count = 0

    def issue(
        self, subject: str, role: str, backend: str, ttl_seconds: int
    ) -> Credential:
        if ttl_seconds <= 0:
            raise BackendError("credential lifetime must be positive")
        self.issue_count += 1
        serial = next(self._ids)
        credential = Credential(
            credential_id=f"cred-{serial:04d}",
            subject=subject,
            role=role,
            backend=backend,
            username=f"{role}_{subject}_{serial}",
            password=f"secret-{serial:04d}",
            issued_at=datetime.now(timezone.utc),
        )
        self._credentials[credential.credential_id] = credential
        return credential

    def revoke(self, credential_id: str) -> bool:
        if credential_id not in self._credentials:
            raise BackendError(f"unknown credential: {credential_id!r}")
        self.revoke_count += 1
        self._revoked.add(credential_id)
        return True

    def is_active(self, credential_id: str) -> bool:
        return (
            credential_id in self._credentials
            and credential_id not in self._revoked
        )


def Revoke(backend: CredentialBackend, credential_id: str) -> bool:
    """Ask ``backend`` to invalidate ``credential_id``.

    Returns ``True`` when the caller may consider the credential revoked.
    Backend failures are never swallowed.
    """

    try:
        revoked = backend.revoke(credential_id)
    except BackendError:
        raise
    except Exception as exc:
        raise BackendError(
            f"backend failed to revoke credential {credential_id!r}: {exc}"
        ) from exc
    if revoked is False:
        raise BackendError(
            f"backend refused to revoke credential {credential_id!r}"
        )
    return True
