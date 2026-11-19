from stowsign.clock import expires_at, now_ts


def test_now_ts_passes_explicit_value_through():
    assert now_ts(1700000000) == 1700000000


def test_now_ts_returns_integer():
    assert isinstance(now_ts(), int)


def test_expires_at_adds_ttl_to_now():
    assert expires_at(3600, now=1700000000) == 1700003600


def test_expires_at_accepts_string_ttl():
    assert expires_at("600", now=1000) == 1600
