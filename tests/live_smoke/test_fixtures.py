from __future__ import annotations

from pathlib import Path

from rail.live_smoke.fixtures import copy_fixture_target, live_smoke_fixture_source


def test_live_smoke_fixture_source_contains_expected_files() -> None:
    source = live_smoke_fixture_source()

    assert (source / "README.md").is_file()
    assert (source / "docs" / "ARCHITECTURE.md").is_file()
    assert (source / "docs" / "CONVENTIONS.md").is_file()
    assert (source / "app" / "service.py").is_file()
    assert (source / "tests" / "test_service.py").is_file()


def test_copy_fixture_target_records_digest_and_excludes_smoke_reports(tmp_path: Path) -> None:
    report_root = tmp_path / "smoke-reports"
    copied = copy_fixture_target(tmp_path / "target", report_root=report_root)

    assert copied.target_root.is_dir()
    assert copied.fixture_digest.startswith("sha256:")
    assert copied.target_root != live_smoke_fixture_source()
    assert not (copied.target_root / "smoke-reports").exists()
