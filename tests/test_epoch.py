"""Epoch bookkeeping tests."""

from __future__ import annotations

from sessforge import epoch


def test_unknown_user_is_at_epoch_zero(store):
    assert epoch.current(store, "alice") == 0


def test_anonymous_has_no_epoch(store):
    assert epoch.current(store, None) == 0


def test_bump_increments(store):
    assert epoch.bump(store, "alice") == 1
    assert epoch.bump(store, "alice") == 2
    assert epoch.current(store, "alice") == 2


def test_bump_is_per_user(store):
    epoch.bump(store, "alice")

    assert epoch.current(store, "bob") == 0


def test_epoch_is_recorded_on_login(store, t0):
    from sessforge.lifecycle import login

    epoch.bump(store, "alice")

    record = login(store, "alice", t0)

    assert record.epoch == 1
