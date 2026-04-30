from __future__ import annotations

import tarfile
import tomllib
import zipfile
from io import BytesIO
from pathlib import Path

from scripts.check_package_asset_alignment import find_alignment_drift
from scripts.check_package_asset_alignment import default_alignment_pairs
from scripts.check_python_package_assets import find_missing_assets


def test_release_gate_runs_asset_alignment_checker():
    gate = Path("scripts/release_gate.sh").read_text(encoding="utf-8")

    assert "scripts/check_package_asset_alignment.py" in gate


def test_distribution_name_is_rail_sdk():
    project = tomllib.loads(Path("pyproject.toml").read_text(encoding="utf-8"))["project"]

    assert project["name"] == "rail-sdk"


def test_installed_wheel_smoke_checks_console_entrypoints():
    script = Path("scripts/check_installed_wheel.py").read_text(encoding="utf-8")

    assert "rail-sdk" in script
    assert "--version" in script
    assert 'policy.runtime.provider == "codex_vault"' in script


def test_release_gate_cleans_current_and_stale_egg_info():
    gate = Path("scripts/release_gate.sh").read_text(encoding="utf-8")

    assert "src/rail_sdk.egg-info" in gate
    assert "src/rail_harness.egg-info" in gate


def test_release_gate_alignment_includes_repo_owned_skill_tree():
    pairs = default_alignment_pairs(Path("."))

    assert (Path("skills/rail"), Path("assets/skill/Rail")) in pairs


def test_asset_alignment_checker_reports_content_drift(tmp_path: Path):
    source = tmp_path / "source"
    packaged = tmp_path / "packaged"
    source.mkdir()
    packaged.mkdir()
    (source / "prompt.md").write_text("source\n", encoding="utf-8")
    (packaged / "prompt.md").write_text("packaged\n", encoding="utf-8")

    drift = find_alignment_drift([(source, packaged)])

    assert drift == [f"{source} != {packaged}: content drift prompt.md"]


def test_packaged_assets_match_repo_sources():
    _assert_tree_matches(Path("assets/defaults"), Path("src/rail/package_assets/defaults"))
    _assert_tree_matches(Path("assets/skill/Rail"), Path("src/rail/package_assets/skill/Rail"))


def test_repo_harness_defaults_match_packaged_defaults():
    for subdir in ("actors", "rules", "rubrics", "supervisor", "templates"):
        _assert_tree_matches(Path(".harness") / subdir, Path("assets/defaults") / subdir)


def test_package_asset_checker_reports_missing_required_assets(tmp_path: Path):
    dist = tmp_path / "dist"
    dist.mkdir()
    _write_wheel(dist / "rail_harness-0.1.0-py3-none-any.whl", ["rail/__init__.py"])
    _write_sdist(dist / "rail_harness-0.1.0.tar.gz", ["rail_harness-0.1.0/pyproject.toml"])

    missing = find_missing_assets(dist, required_assets=tuple(_required_asset_paths()))

    assert "wheel: rail/package_assets/skill/Rail/SKILL.md" in missing
    assert "sdist: src/rail/package_assets/skill/Rail/SKILL.md" in missing
    assert "wheel: rail/package_assets/defaults/supervisor/actor_runtime.yaml" in missing


def test_package_asset_checker_accepts_required_assets(tmp_path: Path):
    dist = tmp_path / "dist"
    dist.mkdir()
    wheel_members = [f"rail/{path}" for path in _required_asset_paths()]
    sdist_members = [f"rail_harness-0.1.0/src/rail/{path}" for path in _required_asset_paths()]
    _write_wheel(dist / "rail_harness-0.1.0-py3-none-any.whl", wheel_members)
    _write_sdist(dist / "rail_harness-0.1.0.tar.gz", sdist_members)

    assert find_missing_assets(dist, required_assets=tuple(_required_asset_paths())) == []


def _required_asset_paths() -> list[str]:
    return [
        "package_assets/skill/Rail/SKILL.md",
        "package_assets/skill/Rail/references/examples.md",
        "package_assets/defaults/actors/planner.md",
        "package_assets/defaults/templates/plan.schema.yaml",
        "package_assets/defaults/supervisor/actor_runtime.yaml",
        "package_assets/defaults/rules/allowed_commands.md",
        "package_assets/defaults/rubrics/bug_fix.yaml",
    ]


def _write_wheel(path: Path, members: list[str]) -> None:
    with zipfile.ZipFile(path, "w") as archive:
        for member in members:
            archive.writestr(member, "content")


def _write_sdist(path: Path, members: list[str]) -> None:
    with tarfile.open(path, "w:gz") as archive:
        for member in members:
            payload = b"content"
            info = tarfile.TarInfo(member)
            info.size = len(payload)
            archive.addfile(info, fileobj=BytesIO(payload))


def _assert_tree_matches(source: Path, packaged: Path) -> None:
    source_files = sorted(path.relative_to(source) for path in source.rglob("*") if path.is_file())
    packaged_files = sorted(path.relative_to(packaged) for path in packaged.rglob("*") if path.is_file())

    assert packaged_files == source_files
    for relative_path in source_files:
        assert (packaged / relative_path).read_text(encoding="utf-8") == (source / relative_path).read_text(encoding="utf-8")
