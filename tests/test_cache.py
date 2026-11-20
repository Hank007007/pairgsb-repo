from rowseal import CompileCache, Context, loads_policy_set
from rowseal.cache import compile_cached


DOCUMENT = """
version: 1
tables:
  orders:
    select:
      - name: everything
        roles: [clerk]
        when: amount > 0
  customers:
    select:
      - name: everything
        roles: [clerk]
        when: region = 'eu'
default:
  select: deny
  update: deny
  delete: deny
"""


def test_second_lookup_is_served_from_the_cache():
    cache = CompileCache()
    policy_set = loads_policy_set(DOCUMENT)
    ctx = Context(role="clerk")

    first = compile_cached(cache, policy_set, "orders", "select", ctx, prefix="orders")
    second = compile_cached(cache, policy_set, "orders", "select", ctx, prefix="orders")

    assert first == second
    assert cache.hits == 1
    assert cache.misses == 1


def test_different_tables_are_cached_separately():
    cache = CompileCache()
    policy_set = loads_policy_set(DOCUMENT)
    ctx = Context(role="clerk")

    orders = compile_cached(cache, policy_set, "orders", "select", ctx, prefix="orders")
    customers = compile_cached(
        cache, policy_set, "customers", "select", ctx, prefix="customers"
    )

    assert orders.sql != customers.sql
    assert cache.misses == 2


def test_eviction_drops_the_least_recently_used_entry():
    cache = CompileCache(capacity=2)
    cache.put("a", 1)
    cache.put("b", 2)
    cache.get("a")
    cache.put("c", 3)
    assert cache.get("b") is None
    assert cache.get("a") == 1
    assert cache.get("c") == 3


def test_clear_empties_the_cache():
    cache = CompileCache()
    cache.put("a", 1)
    cache.clear()
    assert len(cache) == 0
    assert cache.get("a") is None
