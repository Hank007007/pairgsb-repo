"""Credential issuance for approved elevation requests."""

from __future__ import annotations

import uuid
from datetime import datetime
from typing import Optional

from .approval import is_expired
from .errors import CredentialNotAvailable
from .models import Credential, ElevationRequest, RequestStatus
from .policy import Policy


class CredentialLedger:
    """Keeps every credential the service has handed out."""

    def __init__(self) -> None:
        self._by_id: dict[str, Credential] = {}
        self._by_request: dict[str, Credential] = {}

    def record(self, credential: Credential) -> None:
        self._by_id[credential.credential_id] = credential
        self._by_request[credential.request_id] = credential

    def get(self, credential_id: str) -> Optional[Credential]:
        return self._by_id.get(credential_id)

    def for_request(self, request_id: str) -> Optional[Credential]:
        return self._by_request.get(request_id)

    def __len__(self) -> int:
        return len(self._by_id)


def issue(
    request: ElevationRequest,
    policy: Policy,
    now: datetime,
    ledger: CredentialLedger,
) -> Credential:
    """Produce the credential for an approved request."""

    if request.status == RequestStatus.PENDING and is_expired(request, policy, now):
        request.status = RequestStatus.EXPIRED
        raise CredentialNotAvailable(
            f"request {request.request_id} expired before it was approved"
        )

    if request.status != RequestStatus.APPROVED:
        raise CredentialNotAvailable(
            f"request {request.request_id} is {request.status}, not approved"
        )

    existing = ledger.for_request(request.request_id)
    if existing is not None:
        # §6.4: a request yields at most one credential; repeated requests
        # return the original credential instead of extending its lifetime.
        return existing

    if request.approved_at is None:
        raise CredentialNotAvailable(
            f"request {request.request_id} has no recorded approval time"
        )

    # §6.1/§6.2: the lifetime is capped by max_ttl and starts at the moment
    # the request crossed the approval threshold, not at submission time.
    ttl = min(request.duration, policy.max_ttl)
    credential = Credential(
        credential_id=uuid.uuid4().hex,
        request_id=request.request_id,
        subject=request.subject,
        scopes=frozenset(request.scope_at_approval or ()),
        issued_at=request.approved_at,
        expires_at=request.approved_at + ttl,
    )

    ledger.record(credential)
    request.credential_id = credential.credential_id
    return credential
