"""Tests for audience validation."""

from __future__ import annotations

import pytest

from scopebraid import audience
from scopebraid.errors import ExchangeError


def test_string_audience_matching_target() -> None:
    claims = {"aud": "api://warehouse"}
    audience.check(claims, "api://warehouse", ("api://warehouse",))


def test_missing_target_is_rejected() -> None:
    claims = {"aud": "api://warehouse"}
    with pytest.raises(ExchangeError) as excinfo:
        audience.check(claims, "", ("api://warehouse",))
    assert excinfo.value.error == "invalid_target"


def test_mismatched_audience_is_rejected() -> None:
    claims = {"aud": "api://warehouse"}
    with pytest.raises(ExchangeError) as excinfo:
        audience.check(claims, "api://export", ("api://warehouse", "api://export"))
    assert excinfo.value.error == "invalid_target"
