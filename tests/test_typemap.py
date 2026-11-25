"""Type table loading and resolution."""

import pytest

from blobrestore.model.typemap import _in_allowlist, load_type_table, resolve_type

from _blobgen import type_table


def test_load_type_table_normal():
    raw = type_table([(0, "builtins.list"), (1, "builtins.dict")])

    table = load_type_table(raw, 0, len(raw))

    assert table == {0: "builtins.list", 1: "builtins.dict"}


def test_resolve_type_known():
    table = {3: "builtins.tuple"}

    assert resolve_type(table, 3) == "builtins.tuple"


@pytest.mark.parametrize(
    "name",
    [
        "builtins.list",
        "builtins.dict",
        "builtins.tuple",
        "builtins.str",
        "builtins.int",
        "builtins.float",
        "builtins.bool",
        "builtins.NoneType",
    ],
)
def test_allowlist_accepts_builtin_containers(name):
    assert _in_allowlist(name) is True


def test_allowlist_rejects_arbitrary_names():
    assert _in_allowlist("os.system") is False
    assert _in_allowlist("eval") is False
