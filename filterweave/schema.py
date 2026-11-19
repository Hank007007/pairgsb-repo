"""属性表加载与属性名判定。"""

import re
from pathlib import Path

from .errors import UnknownAttributeError

SCHEMA_PATH = Path(__file__).resolve().parent.parent / "docs" / "schema.md"

_BACKTICKED = re.compile(r"`([^`]+)`")


def _load_known():
    names = set()
    try:
        text = SCHEMA_PATH.read_text(encoding="utf-8")
    except OSError:
        return names
    for line in text.splitlines():
        line = line.strip()
        if not line.startswith("|"):
            continue
        first_cell = line.strip("|").split("|")[0].strip()
        match = _BACKTICKED.fullmatch(first_cell)
        if match:
            token = match.group(1).strip()
            if token:
                names.add(token.lower())
    return frozenset(names)


KNOWN = _load_known()


def is_known_attr(name):
    """判断属性名是否在属性表中。"""
    if not isinstance(name, str):
        return False
    return name.lower() in KNOWN


def require_known(name):
    """属性名不在属性表时抛错。"""
    if not is_known_attr(name):
        raise UnknownAttributeError(name)
    return name
