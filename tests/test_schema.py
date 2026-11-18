from filterweave.schema import KNOWN, is_known_attr


def test_known_attribute_is_accepted():
    assert is_known_attr("uid")
    assert is_known_attr("mail")
    assert is_known_attr("departmentNumber")


def test_attribute_lookup_ignores_case():
    assert is_known_attr("UID")
    assert is_known_attr("Departmentnumber")


def test_unknown_attribute_is_rejected():
    assert not is_known_attr("favoriteColor")


def test_empty_attribute_is_rejected():
    assert not is_known_attr("")


def test_known_set_is_not_empty():
    assert len(KNOWN) >= 20
    assert "uid" in KNOWN
