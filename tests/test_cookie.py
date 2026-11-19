"""Cookie attribute tests."""

from __future__ import annotations

from sessforge.cookie import (
    SESSION_COOKIE_NAME,
    SESSION_COOKIE_PATH,
    clearing_cookie,
    session_cookie,
)


def test_session_cookie_name_and_path():
    issued = session_cookie("abc123")

    assert issued.name == SESSION_COOKIE_NAME == "sessforge_session"
    assert issued.path == SESSION_COOKIE_PATH == "/"


def test_session_cookie_security_attributes():
    issued = session_cookie("abc123")

    assert issued.http_only is True
    assert issued.secure is True
    assert issued.same_site == "Lax"


def test_session_cookie_header_contains_flags():
    header = session_cookie("abc123", max_age=3600).to_header()

    assert header.startswith("sessforge_session=abc123")
    assert "Path=/" in header
    assert "Max-Age=3600" in header
    assert "HttpOnly" in header
    assert "Secure" in header
    assert "SameSite=Lax" in header


def test_clearing_cookie_expires_immediately():
    header = clearing_cookie().to_header()

    assert header.startswith("sessforge_session=")
    assert "Max-Age=0" in header
    assert "Path=/" in header
