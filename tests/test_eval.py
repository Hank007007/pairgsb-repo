"""Evaluation tests for ordinary rules and ordinary fields."""

from __future__ import annotations

import pytest

from exprseal import Context, evaluate, parse
from exprseal.context import Request, User
from exprseal.errors import AccessDenied


def run(source: str, context: Context):
    return evaluate(parse(source), context)


def test_plan_and_path_rule(context: Context):
    assert run(
        'user.plan == "pro" and request.path.startswith("/api")', context
    )


def test_equality_on_strings(context: Context):
    assert run('user.role == "admin"', context)
    assert not run('user.role == "owner"', context)
    assert run('user.role != "owner"', context)


def test_numeric_comparison(context: Context):
    assert run("user.age >= 18", context)
    assert run("user.age < 40", context)
    assert not run("user.age > 65", context)


def test_boolean_literal_and_verified(context: Context):
    assert run("user.verified == true", context)
    assert run("user.verified", context) is True


def test_membership_against_list(context: Context):
    assert run('user.role in ["admin", "owner"]', context)
    assert not run('user.role in ["viewer", "guest"]', context)
    assert run("user.role in []", context) is False


def test_membership_of_computed_value(context: Context):
    assert run('lower(user.email) in ["ops@example.com", "root@example.com"]', context)


def test_and_or_short_circuit(context: Context):
    assert run('user.plan == "free" or user.plan == "pro"', context)
    assert not run('user.plan == "free" and user.plan == "pro"', context)


def test_and_returns_the_deciding_operand(context: Context):
    assert run('0 and user.role', context) == 0
    assert run('"x" or user.role', context) == "x"


def test_null_literal(context: Context):
    assert run("request.headers == null", context) is False
    assert run("null == null", context)


def test_arithmetic(context: Context):
    assert run("user.age - 4 * 2", context) == 26
    assert run("(user.age - 4) * 2", context) == 60
    assert run("user.age % 5", context) == 4


def test_unary_minus(context: Context):
    assert run("-user.age + 40", context) == 6


def test_named_whitelisted_functions(context: Context):
    assert run('startswith(request.path, "/api")', context)
    assert run('endswith(request.path, "orders")', context)
    assert run('contains(user.email, "Example")', context)
    assert run("len(user.groups)", context) == 3
    assert run('upper(user.plan) == "PRO"', context)


def test_method_style_whitelisted_functions(context: Context):
    assert run('request.path.startswith("/api")', context)
    assert run('user.email.lower() == "ops@example.com"', context)
    assert run('request.path.endswith("/orders")', context)


def test_field_map_paths(context: Context):
    assert run('user.tenant_id == "acme"', context)
    assert run('request.method == "GET"', context)
    assert run('request.host == "api.example.com"', context)
    assert run('request.ip == "203.0.113.9"', context)
    assert run("request.headers == null", context) is False


def test_unknown_root_name_is_denied(context: Context):
    with pytest.raises(AccessDenied):
        run('tenant_id == "acme"', context)


def test_context_accepts_custom_values():
    user = User(
        id="u_7",
        plan="free",
        role="viewer",
        email="viewer@example.com",
        age=17,
        verified=False,
        tenant_id="beta",
    )
    request = Request(
        path="/health",
        method="POST",
        host="internal.example.com",
        ip="198.51.100.4",
    )
    ctx = Context(user, request)
    assert run('request.path == "/health" and user.plan == "free"', ctx)
    assert run("user.verified", ctx) is False
