"""End-to-end coverage of the ordinary elevation workflow."""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest

from glassbreak import GlassbreakService, load_policy
from glassbreak.errors import CredentialNotAvailable


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


def test_approved_request_yields_a_credential(
    service: GlassbreakService, clock: ManualClock
) -> None:
    request = service.create_request(
        subject="alice",
        requested_scopes=["db:write"],
        reason="replica lag during incident INC-4471",
        duration=timedelta(minutes=45),
    )

    clock.advance(timedelta(minutes=3))
    service.approve(request.request_id, "bob")
    clock.advance(timedelta(minutes=2))
    approved = service.approve(request.request_id, "carol")

    assert approved.is_approved
    assert approved.approvers == ["bob", "carol"]

    issued = service.issue_credential(request.request_id)

    assert issued.request_id == request.request_id
    assert issued.subject == "alice"
    assert "db:write" in issued.scopes
    assert issued.expires_at > issued.issued_at
    assert service.get_credential(issued.credential_id) == issued

    events = [entry.event for entry in service.audit.for_request(request.request_id)]
    assert events == [
        "request.created",
        "request.vote",
        "request.vote",
        "request.approved",
        "credential.issued",
    ]


def test_request_without_enough_votes_gets_no_credential(
    service: GlassbreakService, clock: ManualClock
) -> None:
    request = service.create_request(
        subject="alice",
        requested_scopes=["db:snapshot"],
        reason="need a consistent copy before the migration window",
        duration=timedelta(minutes=30),
    )

    clock.advance(timedelta(minutes=1))
    service.approve(request.request_id, "bob")

    assert not request.is_approved
    assert request.credential_id is None

    with pytest.raises(CredentialNotAvailable):
        service.issue_credential(request.request_id)

    assert service.credential_for(request.request_id) is None
