from filterweave.build import MAX_FILTER_LEN, build_and, build_or, build_present


def test_and_of_two_conditions():
    result = build_and([("uid", "=", "u1001"), ("mail", "=", "a@example.com")])
    assert result == "(&(uid=u1001)(mail=a@example.com))"


def test_and_of_one_condition():
    assert build_and([("uid", "=", "u1001")]) == "(&(uid=u1001))"


def test_and_keeps_value_text_intact():
    result = build_and([("cn", "=", "Alice Smith")])
    assert result == "(&(cn=Alice Smith))"


def test_or_returns_group_wrapping_conditions():
    result = build_or([("uid", "=", "u1001"), ("uid", "=", "u1002")])
    assert result.startswith("(|")
    assert result.endswith(")")
    assert "u1001" in result
    assert "u1002" in result


def test_present_condition_uses_asterisk():
    assert build_present("mail") == "(mail=*)"


def test_present_condition_for_department():
    assert build_present("departmentNumber") == "(departmentNumber=*)"


def test_filter_length_limit_is_exposed():
    assert isinstance(MAX_FILTER_LEN, int)
    assert MAX_FILTER_LEN > 0
