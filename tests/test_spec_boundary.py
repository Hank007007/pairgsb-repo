"""对照 docs/filter-spec.md 的边界用例集。

按 DESIGN.md 第 6 节，转义特殊字符、白名单外属性、超长串、原始片段
通道等边界行为集中在本文件，不与正常路径用例混放。 docstring 中的
§编号引用 docs/filter-spec.md 的条款。
"""

import pytest

from filterweave import (
    MAX_FILTER_LEN,
    FilterTooLongError,
    FilterWeaveError,
    InvalidFilterError,
    UnknownAttributeError,
    build_and,
    build_or,
    build_present,
    escape_dn_value,
    escape_value,
    is_known_attr,
)


# §3 值转义

def test_close_paren_is_escaped():
    assert escape_value("x)") == "x\\29"


def test_backslash_is_escaped():
    assert escape_value("\\") == "\\5c"


def test_nul_is_escaped():
    assert escape_value("\x00") == "\\00"


def test_injection_sample_is_fully_escaped():
    assert escape_value("x)(mail=*)") == "x\\29\\28mail=\\2a\\29"


def test_backslash_is_not_double_escaped():
    # §3.4：输入里的 `\` 只能以 `\5c` 出现，不能产生其它形态。
    assert escape_value("\\2a") == "\\5c2a"


def test_all_five_specials_use_two_digit_lowercase_hex():
    assert escape_value("\\*()\x00") == "\\5c\\2a\\28\\29\\00"


# §3.6 / §5.3 转义后的值不得改变条件边界

def test_build_and_with_hostile_value_keeps_one_condition():
    result = build_and([("uid", "=", "x)(mail=*)")])
    assert result == "(&(uid=x\\29\\28mail=\\2a\\29))"


def test_build_and_with_backslash_and_nul_value():
    result = build_and([("uid", "=", "a\\\x00b")])
    assert result == "(&(uid=a\\5c\\00b))"


# §4 通配符与存在性条件

def test_present_rejects_custom_value():
    with pytest.raises(InvalidFilterError):
        build_present("uid", "x)(mail=*)")


def test_present_custom_value_error_is_filterweave_error():
    with pytest.raises(FilterWeaveError):
        build_present("uid", "x)(mail=*)")


def test_present_validates_attribute_name():
    with pytest.raises(UnknownAttributeError):
        build_present("favoriteColor")


# §5 布尔组合的结构

def test_or_wraps_single_condition_in_parens():
    assert build_or([("uid", "=", "a")]) == "(|(uid=a))"


def test_or_with_pipe_in_value_stays_one_condition():
    assert build_or([("uid", "=", "a|b")]) == "(|(uid=a|b))"


def test_or_wraps_each_condition():
    result = build_or([("uid", "=", "a"), ("mail", "=", "b")])
    assert result == "(|(uid=a)(mail=b))"


def test_empty_conditions_return_constant_filters():
    # §5.4：空集合返回固定的恒真 / 恒假过滤串。
    assert build_and([]) == "(&)"
    assert build_or([]) == "(|)"


# §6 原始片段通道已移除

def test_raw_filters_entry_point_is_removed():
    with pytest.raises(TypeError):
        build_and([("uid", "=", "a")], raw_filters=["(uid=admin)"])


# §2 属性名白名单

def test_wildcard_names_are_not_known():
    assert not is_known_attr("*")
    assert not is_known_attr("objectClass*")


def test_names_with_whitespace_are_not_known():
    assert not is_known_attr("uid ")
    assert not is_known_attr(" uid")
    assert not is_known_attr("u id")


def test_build_and_rejects_unknown_attribute():
    with pytest.raises(UnknownAttributeError):
        build_and([("favoriteColor", "=", "red")])


def test_build_or_rejects_unknown_attribute():
    with pytest.raises(UnknownAttributeError):
        build_or([("favoriteColor", "=", "red")])


def test_unknown_attribute_error_carries_name():
    # §2.4 / §9.2：错误信息必须包含被拒的原始属性名。
    with pytest.raises(UnknownAttributeError) as excinfo:
        build_and([("favoriteColor", "=", "red")])
    assert "favoriteColor" in str(excinfo.value)


def test_build_accepts_attribute_case_insensitively():
    assert build_and([("UID", "=", "a")]) == "(&(UID=a))"


# §7 长度上限

def test_overlong_filter_raises_with_limit():
    with pytest.raises(FilterTooLongError) as excinfo:
        build_and([("uid", "=", "v" * 3000)])
    assert str(MAX_FILTER_LEN) in str(excinfo.value)


def test_filter_at_exact_limit_is_accepted():
    # `(&(uid=` 7 字符 + 值 + `))` 2 字符，总长 = len(value) + 9。
    value = "v" * (MAX_FILTER_LEN - 9)
    result = build_and([("uid", "=", value)])
    assert len(result) == MAX_FILTER_LEN


def test_filter_one_over_limit_is_rejected_not_truncated():
    value = "v" * (MAX_FILTER_LEN - 8)
    with pytest.raises(FilterTooLongError):
        build_and([("uid", "=", value)])


# §8 DN 值转义

def test_dn_value_with_separator_stays_one_rdn():
    assert escape_dn_value("=admin,ou=ops") == "\\=admin\\,ou\\=ops"


def test_dn_escaped_chars():
    assert escape_dn_value(',+"\\<>;=') == "\\,\\+\\\"\\\\\\<\\>\\;\\="


def test_dn_leading_hash_is_escaped():
    assert escape_dn_value("#admin") == "\\#admin"


def test_dn_inner_hash_is_kept():
    assert escape_dn_value("a#b") == "a#b"


def test_dn_leading_and_trailing_spaces_are_escaped():
    assert escape_dn_value(" a ") == "\\ a\\ "
    assert escape_dn_value("  a") == "\\ \\ a"


def test_dn_empty_value_stays_empty():
    assert escape_dn_value("") == ""


def test_dn_built_rdn_keeps_level_count():
    # §8.5：拼进 RDN 后，值里的分隔符不得增加 DN 层级数。
    dn = f"uid={escape_dn_value('=admin,ou=ops')},ou=people"
    assert dn == "uid=\\=admin\\,ou\\=ops,ou=people"
