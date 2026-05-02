from __future__ import annotations

import zipfile
from pathlib import Path

import pytest

from rail.live_smoke import fixtures as fixtures_module
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


def test_copy_fixture_target_replaces_existing_target(tmp_path: Path) -> None:
    target_root = tmp_path / "target"
    target_root.mkdir()
    stale_file = target_root / "stale.txt"
    stale_file.write_text("old fixture content", encoding="utf-8")

    copied = copy_fixture_target(target_root, report_root=tmp_path / "smoke-reports")

    assert not stale_file.exists()
    assert (copied.target_root / "README.md").is_file()
    assert (copied.target_root / "app" / "service.py").is_file()


def test_copy_fixture_target_rejects_source_paths_before_deleting(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    source = live_smoke_fixture_source()

    def fail_if_called(*_args: object, **_kwargs: object) -> None:
        pytest.fail("source guard should reject before deleting or copying")

    monkeypatch.setattr(fixtures_module.shutil, "rmtree", fail_if_called)
    monkeypatch.setattr(fixtures_module.shutil, "copytree", fail_if_called)

    for target_root in (source, source.parent, source / "nested-target"):
        with pytest.raises(ValueError):
            copy_fixture_target(target_root, report_root=tmp_path / "smoke-reports")


def test_copy_fixture_target_does_not_ignore_report_root_basename(tmp_path: Path) -> None:
    copied = copy_fixture_target(tmp_path / "target", report_root=tmp_path / "tests")

    assert (copied.target_root / "tests" / "test_service.py").is_file()


def test_copy_fixture_target_copies_zip_backed_resource(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    archive_path = tmp_path / "fixture.zip"
    with zipfile.ZipFile(archive_path, mode="w") as archive:
        archive.writestr("package_assets/live_smoke/fixture_target/README.md", "# Fixture\n")
        archive.writestr(
            "package_assets/live_smoke/fixture_target/tests/test_service.py",
            "def test_service():\n    pass\n",
        )

    with zipfile.ZipFile(archive_path) as archive:
        zip_root = zipfile.Path(archive)
        monkeypatch.setattr(fixtures_module, "files", lambda _package: zip_root)

        copied = copy_fixture_target(tmp_path / "target", report_root=tmp_path / "reports")

    assert (copied.target_root / "README.md").is_file()
    assert (copied.target_root / "tests" / "test_service.py").is_file()
