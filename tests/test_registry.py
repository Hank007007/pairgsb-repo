from exchangefence.scopes.registry import resolve_alias


def test_resolve_alias_maps_known_alias():
    assert resolve_alias("read") == "docs:read"


def test_resolve_alias_leaves_canonical_name_alone():
    assert resolve_alias("docs:write") == "docs:write"
