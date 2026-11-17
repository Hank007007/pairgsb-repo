"""Approval voting for elevation requests."""

from __future__ import annotations

from datetime import datetime

from .elevate import planned_scope
from .errors import (
    ApproverOutsideGroup,
    DuplicateVoteNotAllowed,
    NamedApproverCannotApprove,
    RequestExpired,
    RequestNotPending,
    SelfApprovalNotAllowed,
)
from .models import ElevationRequest, RequestStatus
from .policy import Policy


def is_expired(request: ElevationRequest, policy: Policy, now: datetime) -> bool:
    """Report whether the request is past its approval window."""

    return now - request.created_at > policy.approval_window


def cast_vote(
    request: ElevationRequest,
    voter: str,
    policy: Policy,
    now: datetime,
) -> ElevationRequest:
    """Record an approval vote and advance the request once the threshold is met."""

    if request.status != RequestStatus.PENDING:
        raise RequestNotPending(f"request {request.request_id} is {request.status}")

    if is_expired(request, policy, now):
        request.status = RequestStatus.EXPIRED
        raise RequestExpired(f"request {request.request_id} expired before approval")

    if voter == request.subject:
        raise SelfApprovalNotAllowed("a request cannot be approved by its subject")

    if policy.group_of(voter) != policy.group_of(request.subject):
        raise ApproverOutsideGroup(
            f"voter {voter!r} is not in the subject's group"
        )

    if request.named_approver is not None and voter == request.named_approver:
        raise NamedApproverCannotApprove(
            f"voter {voter!r} was named in the request and cannot approve it"
        )

    if voter in request.approvers:
        raise DuplicateVoteNotAllowed(
            f"voter {voter!r} has already voted on this request"
        )

    request.approvers.append(voter)

    required = policy.required_votes(request.subject)
    if len(set(request.approvers)) >= required:
        request.status = RequestStatus.APPROVED
        request.approved_at = now
        request.scope_at_approval = frozenset(
            planned_scope(
                policy.base_scopes(request.subject),
                request.requested_scopes,
                policy.grantable,
            )
        )

    return request
