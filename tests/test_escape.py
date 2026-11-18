from filterweave.escape import escape_dn_value, escape_value


def test_plain_ascii_value_is_unchanged():
    assert escape_value("alice") == "alice"


def test_unicode_value_is_unchanged():
    assert escape_value("张伟") == "张伟"
    assert escape_value("José Álvarez") == "José Álvarez"


def test_value_with_space_is_unchanged():
    assert escape_value("Alice Smith") == "Alice Smith"


def test_asterisk_is_escaped():
    assert escape_value("*") == "\\2a"
    assert escape_value("a*b") == "a\\2ab"


def test_open_paren_is_escaped():
    assert escape_value("(") == "\\28"
    assert escape_value("a(b") == "a\\28b"


def test_dn_plain_value_is_unchanged():
    assert escape_dn_value("Alice") == "Alice"
    assert escape_dn_value("张伟") == "张伟"


def test_dn_empty_value_is_unchanged():
    assert escape_dn_value("") == ""


def test_dn_value_with_inner_space_is_unchanged():
    assert escape_dn_value("Alice Smith") == "Alice Smith"
