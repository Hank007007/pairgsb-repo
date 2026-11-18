"""Lexer and parser tests for well-formed input."""

from __future__ import annotations

import pytest

from exprseal import lexer, nodes, parse
from exprseal.errors import ParseError


def kinds(source: str):
    return [token.kind for token in lexer.Lexer(source).tokenize()]


def test_tokenizes_a_rule():
    assert kinds('user.plan == "pro" and not user.verified') == [
        "ident",
        ".",
        "ident",
        "==",
        "string",
        "and",
        "not",
        "ident",
        ".",
        "ident",
        "eof",
    ]


def test_tokenizes_numbers_and_literals():
    tokens = lexer.Lexer("1 2.5 true false null").tokenize()
    assert [(t.kind, t.value) for t in tokens[:-1]] == [
        ("int", 1),
        ("float", 2.5),
        ("true", "true"),
        ("false", "false"),
        ("null", "null"),
    ]


def test_parses_string_escapes():
    tokens = lexer.Lexer(r'"line\nbreak\t\"quoted\""').tokenize()
    assert tokens[0].value == 'line\nbreak\t"quoted"'


def test_parses_list_literal():
    tree = parse('user.role in ["admin", "owner"]')
    assert isinstance(tree, nodes.Binary)
    assert tree.op == "in"
    assert isinstance(tree.right, nodes.ListLiteral)
    assert [item.value for item in tree.right.items] == ["admin", "owner"]


def test_parses_empty_list():
    tree = parse("[]")
    assert isinstance(tree, nodes.ListLiteral)
    assert tree.items == ()


def test_parses_attribute_chain():
    tree = parse("request.path")
    assert isinstance(tree, nodes.Attribute)
    assert tree.name == "path"
    assert isinstance(tree.obj, nodes.Name)
    assert tree.obj.name == "request"


def test_parses_call_arguments():
    tree = parse('startswith(request.path, "/api")')
    assert isinstance(tree, nodes.Call)
    assert isinstance(tree.callee, nodes.Name)
    assert len(tree.args) == 2


def test_parses_method_style_call():
    tree = parse('request.path.startswith("/api")')
    assert isinstance(tree, nodes.Call)
    assert isinstance(tree.callee, nodes.Attribute)
    assert tree.callee.name == "startswith"


def test_and_binds_tighter_than_or():
    tree = parse("a and b or c")
    assert tree.op == "or"
    assert tree.left.op == "and"


def test_arithmetic_precedence():
    tree = parse("1 + 2 * 3")
    assert tree.op == "+"
    assert tree.right.op == "*"


@pytest.mark.parametrize(
    "source",
    [
        "user.plan ==",
        "user.plan = 'pro'",
        "user.plan ! 'pro'",
        '"unterminated',
        '"bad \\q escape"',
        "user..plan",
        "[1, 2,]",
        "(user.plan == 'pro'",
        "1 @ 2",
        "",
    ],
)
def test_rejects_malformed_source(source: str):
    with pytest.raises(ParseError):
        parse(source)
