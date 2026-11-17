"""Shared fixtures for the tplsentry test suite."""

from __future__ import annotations

import os
import sys

import pytest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from tplsentry import TemplateLoader  # noqa: E402

TEMPLATE_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "fixtures", "templates")


@pytest.fixture
def loader() -> TemplateLoader:
    return TemplateLoader(TEMPLATE_DIR)
