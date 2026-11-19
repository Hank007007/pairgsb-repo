"""Regression tests for the findings raised against the 0.1.0 behaviour.

Each test maps to a section of ``docs/session-policy.md`` and an entry in
``docs/incident-reports.md``.  The pre-existing suite did not cover any of
these paths.
"""

from __future__ import annotations

import threading

import pytest

from sessforge import epoch
from sessforge.lifecycle import (
    ABSOLUTE_TIMEOUT_SECONDS,
    IDLE_TIMEOUT_SECONDS,
    MAX_SESSIONS_PER_USER,
    login,
    start_anonymous,
    validate,
)
from sessforge.remember import (
    TokenNotFound,
    TokenReplay,
    issue,
    redeem,
)


# -- section 3: login must rotate the session identifier --------------------

def test_login_rotates_anonymous_sid(store, t0):
    anonymous = start_anonymous(store, t0)

    record = login(store, "alice", t0 + 10, sid=anonymous.sid)

    assert record.sid != anonymous.sid
    assert record.user_id == "alice"
    assert store.get_session(anonymous.sid) is None


def test_pre_login_sid_is_unresolvable_after_login(store, t0):
    anonymous = start_anonymous(store, t0)

    login(store, "alice", t0 + 10, sid=anonymous.sid)

    result = validate(store, anonymous.sid, t0 + 11)
    assert not result.ok
    assert result.reason == "not_found"


def test_login_rotates_even_when_presented_sid_is_authenticated(store, t0):
    first = login(store, "alice", t0)

    second = login(store, "bob", t0 + 10, sid=first.sid)

    assert second.sid != first.sid
    assert store.get_session(first.sid) is None
    assert validate(store, first.sid, t0 + 11).reason == "not_found"


def test_login_without_presented_sid_still_issues_a_session(store, t0):
    record = login(store, "alice", t0)

    assert validate(store, record.sid, t0).ok


# -- section 4: absolute lifetime deadline ----------------------------------

def test_absolute_timeout_expires_session_even_when_kept_warm(store, t0):
    record = login(store, "alice", t0)

    minute = 60
    elapsed = 0
    while elapsed < 13 * 60 * 60:
        elapsed += minute
        result = validate(store, record.sid, t0 + elapsed)
        if elapsed > ABSOLUTE_TIMEOUT_SECONDS:
            assert not result.ok
            assert result.reason == "absolute_timeout"
            return
    pytest.fail("session survived past the 12 hour absolute deadline")


def test_absolute_timeout_boundary_value_is_still_accepted(store, t0):
    record = login(store, "alice", t0)

    # Keep the session warm so the idle deadline cannot mask the absolute one.
    elapsed = 0
    while elapsed < ABSOLUTE_TIMEOUT_SECONDS:
        elapsed += 60
        validate(store, record.sid, t0 + elapsed)

    result = validate(store, record.sid, t0 + ABSOLUTE_TIMEOUT_SECONDS)

    assert result.ok


def test_idle_timeout_boundary_value_is_still_accepted(store, t0):
    record = login(store, "alice", t0)

    result = validate(store, record.sid, t0 + IDLE_TIMEOUT_SECONDS)

    assert result.ok


def test_absolute_timeout_is_not_refreshed_by_activity(store, t0):
    record = login(store, "alice", t0)

    assert validate(store, record.sid, t0 + IDLE_TIMEOUT_SECONDS - 1).ok

    result = validate(store, record.sid, t0 + ABSOLUTE_TIMEOUT_SECONDS + 1)
    assert not result.ok
    assert result.reason == "absolute_timeout"


def test_expired_session_is_not_resurrected_by_later_activity(store, t0):
    record = login(store, "alice", t0)

    validate(store, record.sid, t0 + ABSOLUTE_TIMEOUT_SECONDS + 1)

    assert not validate(store, record.sid, t0 + ABSOLUTE_TIMEOUT_SECONDS + 100).ok


# -- section 5: concurrent session eviction ---------------------------------

def test_sixth_login_is_admitted_and_evicts_idlest_session(store, t0):
    records = [login(store, "alice", t0 + index) for index in range(MAX_SESSIONS_PER_USER)]
    # Keep every session warm except the first one, which stays idle longest.
    for index in range(1, MAX_SESSIONS_PER_USER):
        validate(store, records[index].sid, t0 + 1_000 + index)

    sixth = login(store, "alice", t0 + 2_000)

    assert sixth.sid not in {record.sid for record in records}
    assert len(store.list_user_sessions("alice")) == MAX_SESSIONS_PER_USER
    assert store.get_session(records[0].sid) is None
    assert not validate(store, records[0].sid, t0 + 2_001).ok
    for record in records[1:]:
        assert validate(store, record.sid, t0 + 2_001).ok
    assert validate(store, sixth.sid, t0 + 2_001).ok


def test_eviction_tie_breaks_on_sid_deterministically(store, t0):
    records = [login(store, "alice", t0) for _ in range(MAX_SESSIONS_PER_USER)]
    expected_victim = min(records, key=lambda item: item.sid).sid

    sixth = login(store, "alice", t0)

    assert store.get_session(expected_victim) is None
    assert store.get_session(sixth.sid) is not None
    assert len(store.list_user_sessions("alice")) == MAX_SESSIONS_PER_USER


def test_many_logins_never_refuse_the_user(store, t0):
    records = [login(store, "alice", t0 + index) for index in range(20)]

    assert len(records) == 20
    assert len(store.list_user_sessions("alice")) == MAX_SESSIONS_PER_USER
    assert validate(store, records[-1].sid, t0 + 20).ok


# -- section 6: epochs -------------------------------------------------------

def test_session_recorded_before_epoch_bump_is_rejected(store, t0):
    record = login(store, "alice", t0)

    epoch.bump(store, "alice")

    result = validate(store, record.sid, t0 + 10)
    assert not result.ok
    assert result.reason == "stale_epoch"


def test_session_created_after_epoch_bump_is_accepted(store, t0):
    login(store, "alice", t0)
    epoch.bump(store, "alice")

    record = login(store, "alice", t0 + 10)

    assert validate(store, record.sid, t0 + 20).ok


def test_epoch_bump_revokes_remember_me_tokens(store, t0):
    issued = issue(store, "alice", t0)

    epoch.bump(store, "alice")

    with pytest.raises(TokenNotFound):
        redeem(store, issued.token, t0 + 10)


# -- section 7: remember-me families and replay ------------------------------

def test_redeem_rotates_onto_same_family(store, t0):
    issued = issue(store, "alice", t0)

    successor = redeem(store, issued.token, t0 + 10)

    stored = store.list_remember_tokens("alice")
    assert len(stored) == 1
    assert stored[0].family_id
    assert successor.token != issued.token


def test_replaying_consumed_token_invalidates_the_family(store, t0):
    issued = issue(store, "alice", t0)
    successor = redeem(store, issued.token, t0 + 10)

    with pytest.raises(TokenReplay):
        redeem(store, issued.token, t0 + 20)

    # The minted successor dies with the family.
    with pytest.raises(TokenNotFound):
        redeem(store, successor.token, t0 + 30)
    assert store.list_remember_tokens("alice") == []


def test_unknown_and_family_revoked_tokens_are_indistinguishable(store, t0):
    issued = issue(store, "alice", t0)
    successor = redeem(store, issued.token, t0 + 10)
    with pytest.raises(TokenReplay):
        redeem(store, issued.token, t0 + 20)

    with pytest.raises(TokenNotFound):
        redeem(store, successor.token, t0 + 30)
    with pytest.raises(TokenNotFound):
        redeem(store, "never-issued-token-value", t0 + 30)


def test_concurrent_redemption_has_one_winner_and_a_replay(store, t0):
    issued = issue(store, "alice", t0)
    barrier = threading.Barrier(2)
    outcomes: list[object] = []

    def worker() -> None:
        barrier.wait()
        try:
            outcomes.append(redeem(store, issued.token, t0 + 10))
        except TokenReplay as error:
            outcomes.append(error)

    threads = [threading.Thread(target=worker) for _ in range(2)]
    for thread in threads:
        thread.start()
    for thread in threads:
        thread.join()

    successes = [item for item in outcomes if not isinstance(item, BaseException)]
    replays = [item for item in outcomes if isinstance(item, TokenReplay)]
    assert len(successes) == 1
    assert len(replays) == 1

    # The family is dead: the winner's successor no longer works.
    successor = successes[0]
    with pytest.raises(TokenNotFound):
        redeem(store, successor.token, t0 + 20)


def test_concurrent_redemption_race_repeats_reliably(store, t0):
    for iteration in range(50):
        store.clear()
        issued = issue(store, "alice", t0 + iteration)
        barrier = threading.Barrier(2)
        outcomes: list[tuple[str, object]] = []

        def worker() -> None:
            barrier.wait()
            try:
                outcomes.append(("ok", redeem(store, issued.token, t0 + iteration + 1)))
            except TokenReplay:
                outcomes.append(("replay", None))
            except TokenNotFound:
                outcomes.append(("not_found", None))

        threads = [threading.Thread(target=worker) for _ in range(2)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join()

        kinds = {kind for kind, _ in outcomes}
        assert kinds == {"ok", "replay"}, f"iteration {iteration}: {outcomes}"
