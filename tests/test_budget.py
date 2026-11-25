import pytest

from confseal.intake import Budget


def test_budget_tracks_nodes_and_alias_depth():
    budget = Budget(max_nodes=5, max_alias_depth=2)

    budget.charge()
    budget.charge(3)
    assert budget.nodes == 4

    budget.enter_alias()
    budget.enter_alias()
    assert budget.alias_depth == 2
    budget.leave_alias()
    assert budget.alias_depth == 1

    budget.charge()
    assert budget.nodes == 5

    with pytest.raises(Exception) as caught:
        budget.charge()
    assert getattr(caught.value, "code", None) == "node_limit"
