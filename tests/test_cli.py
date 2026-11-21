"""End-to-end tests for the command line interface."""

from __future__ import annotations

import json

from rotseal.cli import main
from rotseal.util import active_path, segments_dir


def _records(start, count):
    lines = []
    for seq in range(start, start + count):
        lines.append(
            json.dumps(
                {
                    "seq": seq,
                    "ts": "2026-01-01T00:00:00Z",
                    "level": "info",
                    "msg": f"line-{seq}",
                }
            )
        )
    return "\n".join(lines) + "\n"


def test_cli_roundtrip(archive, keyring_file, capsys):
    path = active_path(archive)
    path.write_text(_records(1, 20), encoding="utf-8")

    base = ["--archive", str(archive), "--keyring", str(keyring_file)]
    assert main(base + ["rotate"]) == 0
    assert main(base + ["seal"]) == 0

    out = capsys.readouterr().out
    assert "sealed: segment 1" in out

    assert main(base + ["verify", "--from", "0"]) == 0
    out = capsys.readouterr().out
    assert "ok: segment 1" in out
    assert "chain ok" in out


def test_cli_verify_json_report(archive, keyring_file, capsys):
    active_path(archive).write_text(_records(1, 5), encoding="utf-8")
    base = ["--archive", str(archive), "--keyring", str(keyring_file)]
    main(base + ["rotate"])
    main(base + ["seal"])
    capsys.readouterr()

    assert main(base + ["verify", "--json"]) == 0
    payload = json.loads(capsys.readouterr().out)
    assert payload["ok"] is True
    assert payload["checked"][0]["segment"] == 1


def test_cli_status(archive, keyring_file, capsys):
    active_path(archive).write_text(_records(1, 5), encoding="utf-8")
    base = ["--archive", str(archive), "--keyring", str(keyring_file)]
    main(base + ["rotate"])
    main(base + ["seal"])
    capsys.readouterr()

    assert main(base + ["status"]) == 0
    out = capsys.readouterr().out
    assert "segments: 1" in out
    assert "key-a" in out


def test_cli_key_add_and_list(archive, keyring_file, capsys):
    assert main(["key", "add", "--keyring", str(keyring_file), "--id", "key-z"]) == 0
    capsys.readouterr()
    assert main(["key", "list", "--keyring", str(keyring_file)]) == 0
    out = capsys.readouterr().out
    assert "key-a" in out
    assert "key-z" in out


def test_cli_missing_active_log_is_an_error(archive, keyring_file, capsys):
    code = main(["--archive", str(archive), "rotate"])
    assert code == 2
    assert "no active log file" in capsys.readouterr().err


def test_cli_prune_dry_run(archive, keyring_file, capsys):
    active_path(archive).write_text(_records(1, 5), encoding="utf-8")
    base = ["--archive", str(archive), "--keyring", str(keyring_file)]
    main(base + ["rotate"])
    main(base + ["seal"])
    capsys.readouterr()

    assert main(base + ["prune", "--keep", "30d", "--dry-run"]) == 0
    out = capsys.readouterr().out
    assert "nothing to prune" in out
    assert (segments_dir(archive) / "segment-000001.ndjson").exists()
