"""Normal-path coverage for caveat evaluation."""

from __future__ import annotations

import time

from caveatchain import CaveatContext, is_satisfied


def test_tenant_caveat_matches_exact_tenant() -> None:
    caveat = {"type": "tenant", "tenant": "t-42"}

    assert is_satisfied(caveat, CaveatContext(tenant="t-42"))
    assert not is_satisfied(caveat, CaveatContext(tenant="t-43"))


def test_path_caveat_accepts_paths_below_the_prefix() -> None:
    caveat = {"type": "path", "prefix": "/docs"}

    assert is_satisfied(caveat, CaveatContext(path="/docs/report.txt"))
    assert is_satisfied(caveat, CaveatContext(path="/docs"))


def test_expiry_caveat_accepts_time_before_the_limit() -> None:
    caveat = {"type": "expiry", "exp": time.time() + 60}

    assert is_satisfied(caveat, CaveatContext())
