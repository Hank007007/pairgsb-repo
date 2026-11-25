from pathlib import Path

from confseal import compile_policy, load_document

TESTDATA = Path(__file__).resolve().parent.parent / "testdata"


def test_two_level_policy_compiles():
    text = (TESTDATA / "allow_team_a.yaml").read_text(encoding="utf-8")

    policy = compile_policy(load_document(text))

    assert policy.to_dict() == {
        "version": 1,
        "rules": [
            {
                "id": "allow-team-a",
                "subject": "team-a",
                "action": "allow",
                "destinations": ["partner-a.example", "partner-b.example"],
                "schedule": {"from": "09:00", "until": "17:00"},
                "extra": {},
            }
        ],
    }
