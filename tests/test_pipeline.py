"""Tests for the end-to-end pipeline."""

from syslogfold.pipeline import ingest

MESSAGE = (
    "<34>1 2026-02-11T08:14:02Z acme.example.com authsvc 4411 LOGIN "
    '[origin@32473 ip="10.20.3.9"] user alice logged in'
)


def test_ingest_accepts_standard_message():
    result = ingest(MESSAGE, source="10.20.3.9")
    assert result["ok"] is True
    assert result["tenant"] == "acme"
    assert result["message"]["hostname"] == "acme.example.com"
    assert result["message"]["msg"] == "user alice logged in"
    assert result["cef"].startswith("CEF:0|syslogfold|")


def test_ingest_returns_dedup_key():
    first = ingest(MESSAGE, source="10.20.3.9")
    second = ingest(MESSAGE, source="10.20.3.9")
    assert first["key"]
    assert first["key"] == second["key"]


def test_ingest_reports_duplicate():
    seen = set()
    first = ingest(MESSAGE, source="10.20.3.9", seen=seen)
    second = ingest(MESSAGE, source="10.20.3.9", seen=seen)
    assert first["ok"] is True
    assert second["ok"] is False
    assert second["error"]["field"] == "dedup"


def test_ingest_accepts_bytes():
    result = ingest(MESSAGE.encode("utf-8"), source="10.20.3.9")
    assert result["ok"] is True
    assert result["message"]["hostname"] == "acme.example.com"
