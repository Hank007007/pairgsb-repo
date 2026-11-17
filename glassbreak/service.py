"""Service facade tying requests, approvals, credentials and audit together."""

from __future__ import annotations

import uuid
from datetime import datetime, timedelta, timezone
from typing import Callable, Iterable, Optional

from . import credential as credential_module
from .approval import cast_vote
from .audit import AuditLog
from .errors import InvalidRequest, RequestNotFound, UnknownPrincipal
from .models import Credential, ElevationRequest
from .policy import Policy, load_policy

Clock = Callable[[], datetime]


def _utc_now() -> datetime:
    return datetime.now(timezone.utc)


class GlassbreakService:
    """In-memory break-glass workflow."""

    def __init__(
        self,
        policy: Optional[Policy] = None,
        clock: Optional[Clock] = None,
    ) -> None:
        self.policy = policy if policy is not None else load_policy()
        self._clock = clock if clock is not None else _utc_now
        self._requests: dict[str, ElevationRequest] = {}
        self._ledger = credential_module.CredentialLedger()
        self.audit = AuditLog()

    # -- requests ---------------------------------------------------------

    def create_request(
        self,
        subject: str,
        requested_scopes: Iterable[str],
        reason: str,
        duration: timedelta,
        named_approver: Optional[str] = None,
    ) -> ElevationRequest:
        """Register a new elevation request."""

        if self.policy.group_of(subject) is None:
            raise UnknownPrincipal(f"principal {subject!r} is not in any group")
        if not reason.strip():
            raise InvalidRequest("reason must not be empty")
        if duration <= timedelta(0):
            raise InvalidRequest("duration must be positive")

        now = self._clock()
        request = ElevationRequest(
            request_id=uuid.uuid4().hex,
            subject=subject,
            requested_scopes=frozenset(requested_scopes),
            reason=reason.strip(),
            duration=duration,
            created_at=now,
            named_approver=named_approver,
        )
        self._requests[request.request_id] = request
        self.audit.record(
            "request.created",
            at=now,
            request_id=request.request_id,
            actor=subject,
            requested_scopes=sorted(request.requested_scopes),
            duration_seconds=duration.total_seconds(),
            named_approver=named_approver,
        )
        return request

    def get_request(self, request_id: str) -> ElevationRequest:
        try:
            return self._requests[request_id]
        except KeyError:
            raise RequestNotFound(f"unknown request {request_id!r}") from None

    def list_requests(self) -> list[ElevationRequest]:
        return list(self._requests.values())

    # -- approval ---------------------------------------------------------

    def approve(self, request_id: str, voter: str) -> ElevationRequest:
        """Cast an approval vote from ``voter``."""

        request = self.get_request(request_id)
        now = self._clock()
        cast_vote(request, voter, self.policy, now)
        self.audit.record(
            "request.vote",
            at=now,
            request_id=request.request_id,
            actor=voter,
            status=request.status.value,
            vote_count=len(request.approvers),
        )
        if request.is_approved:
            self.audit.record(
                "request.approved",
                at=now,
                request_id=request.request_id,
                actor=voter,
                scope=sorted(request.scope_at_approval or ()),
            )
        return request

    # -- credentials ------------------------------------------------------

    def issue_credential(self, request_id: str) -> Credential:
        """Issue the credential for an approved request."""

        request = self.get_request(request_id)
        now = self._clock()
        issued = credential_module.issue(request, self.policy, now, self._ledger)
        self.audit.record(
            "credential.issued",
            at=now,
            request_id=request.request_id,
            actor=request.subject,
            credential_id=issued.credential_id,
            scope=sorted(issued.scopes),
            expires_at=issued.expires_at.isoformat(),
        )
        return issued

    def get_credential(self, credential_id: str) -> Optional[Credential]:
        return self._ledger.get(credential_id)

    def credential_for(self, request_id: str) -> Optional[Credential]:
        return self._ledger.for_request(request_id)

    @property
    def credential_count(self) -> int:
        return len(self._ledger)
