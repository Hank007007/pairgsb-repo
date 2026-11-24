from exchangefence.scopes import canon


def test_normalize_splits_on_whitespace():
    assert canon.normalize("docs:read docs:write") == ["docs:read", "docs:write"]


def test_normalize_accepts_a_list_of_strings():
    assert canon.normalize(["docs:read", "docs:write"]) == ["docs:read", "docs:write"]


def test_normalize_returns_empty_for_missing_scope():
    assert canon.normalize(None) == []


def test_covers_accepts_a_narrower_request():
    assert canon.covers("docs:read docs:write", "docs:read") is True


def test_covers_rejects_a_missing_scope():
    assert canon.covers("docs:read", "docs:write") is False
