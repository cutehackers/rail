from __future__ import annotations

import rail


def test_public_api_exports_harness_operations():
    assert callable(rail.specify)
    assert not hasattr(rail, "normalize_request")
    assert callable(rail.start_task)
    assert callable(rail.supervise)
    assert callable(rail.status)
    assert callable(rail.result)
