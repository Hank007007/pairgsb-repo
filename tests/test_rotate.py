import pytest

from tokengrit import RotationError, Store, rotate, verify_token


@pytest.fixture()
def store():
    s = Store()
    yield s
    s.close()


def test_rotate_issues_successor_in_same_family(store):
    issued = store.issue(scopes=["read"], now=1000.0)
    successor = rotate(store, issued.raw, now=1060.0)
    assert successor.token_id != issued.token_id
    assert successor.family_id == issued.family_id
    assert successor.raw != issued.raw


def test_successor_verifies(store):
    issued = store.issue(scopes=["read"], now=1000.0)
    successor = rotate(store, issued.raw, now=1060.0)
    result = verify_token(store, successor.raw, now=1070.0)
    assert result.ok
    assert result.family_id == issued.family_id


def test_predecessor_stays_usable_during_grace(store):
    issued = store.issue(scopes=["read"], now=1000.0)
    rotate(store, issued.raw, now=1060.0)
    assert verify_token(store, issued.raw, now=1060.0 + 3600.0).ok


def test_rotation_rejects_invalid_predecessor(store):
    with pytest.raises(RotationError):
        rotate(store, "not-a-token", now=1000.0)
