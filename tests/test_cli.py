"""Tests for the command line entry point."""

from __future__ import annotations

import json

from privpath.cli import main


def _write(path, document):
    path.write_text(json.dumps(document), encoding="utf-8")
    return path


def _config(rules, principal="alice", role="r"):
    return {
        "roles": {role: {"rules": rules}},
        "bindings": {principal: [role]},
        "principals": [principal],
    }


def test_clean_diff_exits_zero(tmp_path, capsys):
    before = _write(
        tmp_path / "before.json",
        _config([{"actions": ["read"], "resources": ["docs/a"]}]),
    )
    after = _write(
        tmp_path / "after.json",
        _config([{"actions": ["read"], "resources": ["docs/a"]}]),
    )
    assert main([str(before), str(after)]) == 0
    assert "total: 0" in capsys.readouterr().out


def test_added_grant_is_printed(tmp_path, capsys):
    before = _write(tmp_path / "before.json", _config([]))
    after = _write(
        tmp_path / "after.json",
        _config([{"actions": ["read"], "resources": ["docs/a"]}]),
    )
    assert main([str(before), str(after)]) == 0
    out = capsys.readouterr().out
    assert "add alice read docs/a via r" in out


def test_usage_error_on_missing_file(tmp_path):
    missing = tmp_path / "missing.json"
    after = _write(tmp_path / "after.json", _config([]))
    assert main([str(missing), str(after)]) == 2


def test_labels_appear_in_header(tmp_path, capsys):
    before = _write(tmp_path / "before.json", _config([]))
    after = _write(tmp_path / "after.json", _config([]))
    main([str(before), str(after), "--before-label", "main", "--after-label", "pr"])
    out = capsys.readouterr().out
    assert "before: main" in out
    assert "after: pr" in out
