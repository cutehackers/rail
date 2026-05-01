from __future__ import annotations

import pytest

from rail.artifacts.run_attempts import allocate_run_attempt


def test_allocate_run_attempt_uses_monotonic_refs(tmp_path):
    assert allocate_run_attempt(tmp_path) == "attempt-0001"
    assert allocate_run_attempt(tmp_path) == "attempt-0002"
    assert (tmp_path / "runs" / "attempt-0001").is_dir()
    assert (tmp_path / "runs" / "attempt-0002").is_dir()


def test_allocate_run_attempt_rejects_unsafe_existing_path(tmp_path):
    runs = tmp_path / "runs"
    runs.mkdir()
    (runs / "attempt-0001").symlink_to(tmp_path)

    with pytest.raises(ValueError, match="unsafe run attempt"):
        allocate_run_attempt(tmp_path)
