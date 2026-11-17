"""Core value types for the glassbreak elevation service."""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timedelta
from enum import Enum
from typing import Optional


class RequestStatus(str, Enum):
    """Lifecycle of an elevation request."""

    PENDING = "pending"
    APPROVED = "approved"
    EXPIRED = "expired"


@dataclass
class ElevationRequest:
    """A break-glass request raised by an operator.

    ``requested_scopes`` holds only the scopes the operator asked to *add* on
    top of their baseline access.  ``scope_at_approval`` is the scope set that
    was in force at the moment the request crossed its vote threshold.
    """

    request_id: str
    subject: str
    requested_scopes: frozenset[str]
    reason: str
    duration: timedelta
    created_at: datetime
    named_approver: Optional[str] = None
    status: RequestStatus = RequestStatus.PENDING
    approvers: list[str] = field(default_factory=list)
    approved_at: Optional[datetime] = None
    scope_at_approval: Optional[frozenset[str]] = None
    credential_id: Optional[str] = None

    @property
    def is_approved(self) -> bool:
        return self.status == RequestStatus.APPROVED

    @property
    def is_expired(self) -> bool:
        return self.status == RequestStatus.EXPIRED


@dataclass(frozen=True)
class Credential:
    """A short-lived credential handed out after an approved request."""

    credential_id: str
    request_id: str
    subject: str
    scopes: frozenset[str]
    issued_at: datetime
    expires_at: datetime

    def is_valid_at(self, moment: datetime) -> bool:
        return self.issued_at <= moment < self.expires_at
