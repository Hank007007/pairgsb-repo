from credlint.redact import redact_url

BLOB = "8f3c1d9a7b2e6f405c8d1a3b5e7f9012"


def test_redact_url_masks_path_segment():
    url = f"https://cdn.example.com/builds/{BLOB}/artifact.tar.gz"
    redacted = redact_url(url)

    assert BLOB not in redacted
    assert redacted.startswith("https://cdn.example.com/builds/")
    assert redacted.endswith("/artifact.tar.gz")
