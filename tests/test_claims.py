"""Tests for claim construction."""

from __future__ import annotations

from scopebraid import claims


def test_downstream_claims_contain_required_fields() -> None:
    built = claims.build_downstream_claims(
        subject="user-1",
        audience="api://warehouse",
        scopes=["datasets:read"],
        client_id="svc-analytics",
        actor="svc-analytics",
        parent_claims={"jti": "parent-jti"},
        issued_at=1_700_000_000.0,
        lifetime=300,
        jti="new-jti",
    )

    assert built["iss"] == "scopebraid"
    assert built["sub"] == "user-1"
    assert built["aud"] == "api://warehouse"
    assert built["scope"] == "datasets:read"
    assert built["client_id"] == "svc-analytics"
    assert built["token_use"] == "access"
    assert built["iat"] == 1_700_000_000.0
    assert built["nbf"] == 1_700_000_000.0
    assert built["exp"] == 1_700_000_300.0
    assert built["jti"] == "new-jti"


def test_act_names_the_actor_and_parent_token() -> None:
    act = claims.build_act({"jti": "parent-jti"}, "svc-analytics")
    assert act["sub"] == "svc-analytics"
    assert act["jti"] == "parent-jti"
