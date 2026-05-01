from __future__ import annotations

from pathlib import Path

import rail


def test_public_api_exports_harness_operations():
    assert callable(rail.specify)
    assert not hasattr(rail, "normalize_request")
    assert callable(rail.start_task)
    assert callable(rail.supervise)
    assert callable(rail.status)
    assert callable(rail.result)


def test_request_version_validator_uses_validate_naming():
    schema_source = Path("src/rail/request/schema.py").read_text(encoding="utf-8")

    assert "_validate_request_version" in schema_source
    assert "_normalize_request_version" not in schema_source
