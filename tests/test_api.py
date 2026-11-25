from pathlib import Path

from confseal import handle_upload

TESTDATA = Path(__file__).resolve().parent.parent / "testdata"


def test_bad_yaml_returns_400():
    text = (TESTDATA / "broken.yaml").read_text(encoding="utf-8")

    result = handle_upload(text)

    assert result["ok"] is False
    assert result["status"] == 400
    assert result["policy"] is None
    assert result["error"]["code"] == "invalid_yaml"
    assert isinstance(result["error"]["line"], int)
