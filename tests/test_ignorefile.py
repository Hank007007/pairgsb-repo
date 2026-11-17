from credlint.ignorefile import IgnoreFile


def test_allowlist_pattern_allows_matching_file():
    allowlist = IgnoreFile.from_lines(["# generated docs", "", "docs/examples/*.md"])

    assert allowlist.matches("docs/examples/setup.md")
    assert not allowlist.matches("src/app.py")
