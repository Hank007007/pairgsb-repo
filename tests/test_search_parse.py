"""Straight-path tests for criteria validation."""

import pytest

from imapquill.errors import BadRequest
from imapquill.search.parse import validate_criteria


def test_validate_accepts_a_plain_object():
    payload = {"from": "alice@example.com", "unseen": True}
    assert validate_criteria(payload) == payload


def test_validate_accepts_dates_and_sizes():
    payload = {"since": "2024-01-05", "larger": 1024}
    assert validate_criteria(payload) == payload


def test_validate_rejects_a_non_object():
    with pytest.raises(BadRequest):
        validate_criteria(["subject"])
