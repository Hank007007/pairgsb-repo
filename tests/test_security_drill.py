"""Regression coverage for the issues found in the security drill.

The scenarios mirror ``docs/elevation-rules.md`` sections 2-7: grantable
filtering, voter eligibility, distinct-vote counting, TTL capping and the
single-credential guarantee.
"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest

from glassbreak import GlassbreakService, load_policy
from glassbreak.elevate import planned_scope
from glassbreak.errors import (
    ApproverOutsideGroup,
    CredentialNotAvailable,
    DuplicateVoteNotAllowed,
    NamedApproverCannotApprove,
    RequestExpired,
    SelfApprovalNotAllowed,
    VoteRejected,
)


class ManualClock:
    def __init__(self, start: datetime) -> None:
        self.current = start

    def __call__(self) -> datetime:
        return self.current

    def advance(self, delta: timedelta) -> None:
        self.current += delta


@pytest.fixture()
def clock() -> ManualClock:
    return ManualClock(datetime(2026, 3, 14, 9, 0, tzinfo=timezone.utc))


@pytest.fixture()
def service(clock: ManualClock) -> GlassbreakService:
    return GlassbreakService(load_policy(), clock=clock)


def _approve_with(service: GlassbreakService, request_id: str, voters: list[str]) -> None:
    for voter in voters:
        service.approve(request_id, voter)


# -- §2 scope planning ------------------------------------------------------


def test_wildcard_scope_not_in_catalogue_is_filtered_out(
    service: GlassbreakService,
) -> None:
    request = service.create_request(
        subject="alice",
        requested_scopes=["db:write", "iam:*"],
        reason="drill",
        duration=timedelta(minutes=30),
    )

    _approve_with(service, request.request_id, ["bob", "carol"])
    credential = service.issue_credential(request.request_id)

    assert "iam:*" not in credential.scopes
    assert "db:write" in credential.scopes
    # Baseline scopes survive even though they are not in the grantable list.
    assert {"logs:read", "metrics:read"} <= credential.scopes
    assert request.scope_at_approval == frozenset(credential.scopes)


def test_wildcard_only_takes_effect_when_listed_verbatim() -> None:
    base = ["db:read"]

    without_listing = planned_scope(base, ["iam:*"], grantable={"db:write"})
    assert without_listing == {"db:read"}

    with_listing = planned_scope(base, ["iam:*"], grantable={"iam:*"})
    assert with_listing == {"db:read", "iam:*"}


# -- §3 voter eligibility ---------------------------------------------------


def test_duplicate_vote_from_same_approver_is_rejected(
    service: GlassbreakService,
) -> None:
    request = service.create_request(
        subject="alice",
        requested_scopes=["db:write"],
        reason="drill",
        duration=timedelta(minutes=30),
    )

    service.approve(request.request_id, "bob")
    with pytest.raises(DuplicateVoteNotAllowed):
        service.approve(request.request_id, "bob")

    assert not request.is_approved
    assert request.approvers == ["bob"]

    # A second distinct approver still crosses the two-vote threshold.
    approved = service.approve(request.request_id, "carol")
    assert approved.is_approved
    assert approved.approvers == ["bob", "carol"]


def test_cross_group_vote_is_rejected(service: GlassbreakService) -> None:
    request = service.create_request(
        subject="alice",
        requested_scopes=["db:write"],
        reason="drill",
        duration=timedelta(minutes=30),
    )

    # erin belongs to dba; alice belongs to sre.
    with pytest.raises(ApproverOutsideGroup):
        service.approve(request.request_id, "erin")

    assert request.approvers == []
    assert not request.is_approved

    _approve_with(service, request.request_id, ["bob", "carol"])
    assert request.is_approved


def test_named_approver_cannot_vote_but_other_members_can(
    service: GlassbreakService,
) -> None:
    request = service.create_request(
        subject="alice",
        requested_scopes=["db:write"],
        reason="drill",
        duration=timedelta(minutes=30),
        named_approver="dave",
    )

    with pytest.raises(NamedApproverCannotApprove):
        service.approve(request.request_id, "dave")
    assert request.approvers == []

    _approve_with(service, request.request_id, ["bob", "carol"])
    assert request.is_approved
    assert request.approvers == ["bob", "carol"]


def test_subject_cannot_approve_own_request(service: GlassbreakService) -> None:
    request = service.create_request(
        subject="alice",
        requested_scopes=["db:write"],
        reason="drill",
        duration=timedelta(minutes=30),
    )

    with pytest.raises(SelfApprovalNotAllowed):
        service.approve(request.request_id, "alice")
    assert request.approvers == []


def test_rejected_votes_are_still_audited(service: GlassbreakService) -> None:
    request = service.create_request(
        subject="alice",
        requested_scopes=["db:write"],
        reason="drill",
        duration=timedelta(minutes=30),
    )

    for voter, error in (
        ("alice", SelfApprovalNotAllowed),
        ("erin", ApproverOutsideGroup),
        ("bob", None),
        ("bob", DuplicateVoteNotAllowed),
    ):
        if error is None:
            service.approve(request.request_id, voter)
        else:
            with pytest.raises(VoteRejected) as exc_info:
                service.approve(request.request_id, voter)
            assert isinstance(exc_info.value, error)

    vote_events = [
        entry
        for entry in service.audit.for_request(request.request_id)
        if entry.event == "request.vote"
    ]
    assert [entry.actor for entry in vote_events] == ["alice", "erin", "bob", "bob"]
    assert request.approvers == ["bob"]


# -- §5 approval window -----------------------------------------------------


def test_vote_and_issuance_after_window_are_rejected(
    service: GlassbreakService, clock: ManualClock
) -> None:
    request = service.create_request(
        subject="alice",
        requested_scopes=["db:write"],
        reason="drill",
        duration=timedelta(minutes=30),
    )

    clock.advance(timedelta(minutes=31))
    with pytest.raises(RequestExpired):
        service.approve(request.request_id, "bob")

    assert request.is_expired
    with pytest.raises(CredentialNotAvailable):
        service.issue_credential(request.request_id)
    assert service.credential_for(request.request_id) is None


# -- §6 credential lifetime and uniqueness ----------------------------------


def test_ttl_is_capped_by_max_ttl_and_anchored_at_approval(
    service: GlassbreakService, clock: ManualClock
) -> None:
    request = service.create_request(
        subject="alice",
        requested_scopes=["db:write"],
        reason="drill",
        duration=timedelta(hours=8),
    )

    clock.advance(timedelta(minutes=10))
    service.approve(request.request_id, "bob")
    clock.advance(timedelta(minutes=2))
    service.approve(request.request_id, "carol")

    assert request.approved_at == request.created_at + timedelta(minutes=12)

    credential = service.issue_credential(request.request_id)

    # max_ttl_minutes is 120; the 8-hour request must be capped, and the
    # lifetime starts at the approval moment rather than submission.
    assert credential.issued_at == request.approved_at
    assert credential.expires_at - request.approved_at == timedelta(minutes=120)
    assert credential.expires_at - request.created_at == timedelta(minutes=132)


def test_request_shorter_than_max_ttl_keeps_its_duration(
    service: GlassbreakService, clock: ManualClock
) -> None:
    request = service.create_request(
        subject="alice",
        requested_scopes=["db:write"],
        reason="drill",
        duration=timedelta(minutes=45),
    )

    clock.advance(timedelta(minutes=10))
    _approve_with(service, request.request_id, ["bob", "carol"])
    credential = service.issue_credential(request.request_id)

    assert credential.expires_at - request.approved_at == timedelta(minutes=45)


def test_repeated_issuance_returns_the_same_credential(
    service: GlassbreakService,
) -> None:
    request = service.create_request(
        subject="alice",
        requested_scopes=["db:write"],
        reason="drill",
        duration=timedelta(minutes=30),
    )
    _approve_with(service, request.request_id, ["bob", "carol"])

    first = service.issue_credential(request.request_id)
    second = service.issue_credential(request.request_id)

    assert second is first
    assert second.credential_id == first.credential_id
    assert second.scopes == first.scopes
    assert second.expires_at == first.expires_at
    assert service.credential_count == 1
    assert service.credential_for(request.request_id) == first

    issuance_events = [
        entry
        for entry in service.audit.for_request(request.request_id)
        if entry.event == "credential.issued"
    ]
    assert len(issuance_events) == 1


def test_credential_cannot_be_issued_before_approval(service: GlassbreakService) -> None:
    request = service.create_request(
        subject="alice",
        requested_scopes=["db:write"],
        reason="drill",
        duration=timedelta(minutes=30),
    )
    service.approve(request.request_id, "bob")

    with pytest.raises(CredentialNotAvailable):
        service.issue_credential(request.request_id)
    assert service.credential_count == 0
