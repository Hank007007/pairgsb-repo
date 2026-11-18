"""Regression tests for the isolation and resource-limit review findings."""

from __future__ import annotations

import pytest

from exprseal import Context, evaluate, nodes, parse
from exprseal.context import Request, User
from exprseal.errors import (
    AccessDenied,
    EvaluationError,
    LimitExceeded,
    ParseError,
)
from exprseal.lexer import Token
from exprseal.parser import Parser


@pytest.fixture()
def min_context() -> Context:
    return Context(
        User(
            id="u_min",
            plan="pro",
            role="min",
            email="min@example.com",
            age=20,
            verified=True,
            tenant_id="acme",
            groups=("min", "oncall"),
        ),
        Request(
            path="/api/v1/orders",
            method="GET",
            host="api.example.com",
            ip="203.0.113.9",
            headers={"accept": "application/json"},
        ),
    )


def run(source: str, context: Context):
    return evaluate(parse(source), context)


@pytest.mark.parametrize(
    "source",
    [
        "user.__dict__",
        "user.__init__",
        'user.__getattribute__("plan")',
        "user.nope",
        "user.__class__",
        "user.__globals__",
        "user.__subclasses__()",
        "request.headers.items()",
    ],
)
def test_hidden_or_unknown_attributes_are_denied(source: str, min_context: Context):
    with pytest.raises(AccessDenied):
        run(source, min_context)


@pytest.mark.parametrize(
    "source",
    [
        'request.path.split("/")',
        'request.path.replace("api", "v2")',
        'request.path.replace("api", "v2").split("/")',
        'request.path.upper().split(" ")',
        'user.plan.removeprefix("p")',
    ],
)
def test_non_whitelisted_methods_are_denied(source: str, min_context: Context):
    with pytest.raises(AccessDenied):
        run(source, min_context)


def test_whitelisted_method_style_calls_still_work(min_context: Context):
    assert run('request.path.startswith("/api")', min_context) is True
    assert run('user.email.lower() == "min@example.com"', min_context) is True
    assert run('request.path.endswith("/orders")', min_context) is True


def test_not_applies_to_the_whole_comparison(min_context: Context):
    assert run('not user.role in ["admin"]', min_context) is True
    assert run('not user.role == "admin"', min_context) is True
    assert run('not user.role in ["min"]', min_context) is False
    assert run('not user.role == "min"', min_context) is False
    assert run("(not user.role) in [false]", min_context) is True


@pytest.mark.parametrize(
    "source",
    [
        'user.role in "admin"',
        '"min" in "admin"',
        "user.role in request.headers",
        "user.role in 1",
    ],
)
def test_membership_requires_a_list_right_operand(
    source: str, min_context: Context
):
    with pytest.raises(EvaluationError):
        run(source, min_context)


def test_membership_against_lists_and_user_groups(min_context: Context):
    assert run('user.role in ["admin", "min"]', min_context) is True
    assert run('"min" in ["min"]', min_context) is True
    assert run("user.role in user.groups", min_context) is True
    assert run("user.role in []", min_context) is False


@pytest.mark.parametrize(
    "build_source",
    [
        lambda: "1 " * 300,
        lambda: "[" + ",".join(["0"] * 100_000) + "]",
        lambda: "(" * 3000 + "1" + ")" * 3000,
        lambda: "1" + " " * 2_000_000 + "1",
    ],
    ids=["600-bytes", "100k-list", "3000-parens", "2MB"],
)
def test_source_size_limit_is_enforced(build_source):
    with pytest.raises(LimitExceeded):
        parse(build_source())


def test_source_size_is_measured_in_utf8_bytes():
    with pytest.raises(LimitExceeded):
        parse("\u00e9" * 300)


@pytest.mark.parametrize(
    "source",
    ["\u53d8\u91cf == 1", "\u00e9 == 1", '"line\rbreak"'],
)
def test_lexer_rejects_non_spec_identifiers_and_newlines(source: str):
    with pytest.raises(ParseError):
        parse(source)


@pytest.mark.parametrize("source", [None, 123, b"user.plan"])
def test_source_must_be_a_string(source):
    with pytest.raises(ParseError):
        parse(source)


def test_token_limit_is_enforced_before_parsing():
    tokens = [Token("int", 1, 0)] * 2049 + [Token("eof", None, 0)]
    with pytest.raises(LimitExceeded):
        Parser(tokens).parse()


def test_ast_depth_limit_is_enforced_after_parsing():
    with pytest.raises(LimitExceeded):
        parse("not " * 33 + "true")


def test_ast_depth_limit_is_enforced_before_direct_evaluation(
    min_context: Context,
):
    tree: nodes.Node = nodes.Literal(True, 0)
    for _ in range(33):
        tree = nodes.Unary("not", tree, 0)
    with pytest.raises(LimitExceeded):
        evaluate(tree, min_context)


def test_evaluation_step_limit_stops_large_trees(min_context: Context):
    items = tuple(nodes.Literal(value, 0) for value in range(10_001))
    with pytest.raises(LimitExceeded):
        evaluate(nodes.ListLiteral(items, 0), min_context)
