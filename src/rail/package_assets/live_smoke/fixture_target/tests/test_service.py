from app.service import normalize_title


def test_normalize_title() -> None:
    assert normalize_title(" rail   smoke ") == "Rail Smoke"
