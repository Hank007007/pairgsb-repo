"""Shared fixtures for the actrail test-suite."""

from __future__ import annotations

import json
import sys
from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).resolve().parent.parent
if str(REPO_ROOT) not in sys.path:
    sys.path.insert(0, str(REPO_ROOT))

from actrail.audit import AuditStore  # noqa: E402
from actrail.clock import FixedClock  # noqa: E402
from actrail.policy import Policy  # noqa: E402

TEST_KEY = b"test-key-for-actrail"
EPOCH = 1_700_000_000


@pytest.fixture()
def key() -> bytes:
    return TEST_KEY


@pytest.fixture()
def clock() -> FixedClock:
    return FixedClock(EPOCH)


@pytest.fixture()
def store() -> AuditStore:
    return AuditStore()


@pytest.fixture()
def policy() -> Policy:
    return Policy.load()


@pytest.fixture()
def policy_documents():
    """Return the raw binding and principal documents."""

    bindings = json.loads((REPO_ROOT / "policy" / "bindings.json").read_text("utf-8"))
    principals = json.loads(
        (REPO_ROOT / "policy" / "principals.json").read_text("utf-8")
    )
    return bindings, principals
