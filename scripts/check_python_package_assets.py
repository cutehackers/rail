#!/usr/bin/env python3
from __future__ import annotations

import sys
import tarfile
import zipfile
from pathlib import Path


REQUIRED_PACKAGE_ASSETS = (
    "package_assets/skill/Rail/SKILL.md",
    "package_assets/skill/Rail/references/examples.md",
    "package_assets/defaults/actors/planner.md",
    "package_assets/defaults/templates/plan.schema.yaml",
    "package_assets/defaults/supervisor/actor_runtime.yaml",
    "package_assets/defaults/rules/allowed_commands.md",
    "package_assets/defaults/rubrics/bug_fix.yaml",
)


def find_missing_assets(dist_dir: str | Path, required_assets: tuple[str, ...] | None = None) -> list[str]:
    dist_path = Path(dist_dir)
    assets = required_assets if required_assets is not None else _required_assets_from_source()
    wheel = _single_archive(dist_path, "*.whl")
    sdist = _single_archive(dist_path, "*.tar.gz")
    missing: list[str] = []

    if wheel is None:
        missing.append("wheel: <missing archive>")
    else:
        wheel_members = _wheel_members(wheel)
        for asset in assets:
            expected = f"rail/{asset}"
            if expected not in wheel_members:
                missing.append(f"wheel: {expected}")

    if sdist is None:
        missing.append("sdist: <missing archive>")
    else:
        sdist_members = _sdist_members(sdist)
        for asset in assets:
            expected = f"src/rail/{asset}"
            if not _sdist_contains(sdist_members, expected):
                missing.append(f"sdist: {expected}")

    return missing


def main(argv: list[str]) -> int:
    dist_dir = Path(argv[1]) if len(argv) > 1 else Path("dist")
    missing = find_missing_assets(dist_dir)
    if missing:
        print("Missing required package assets:", file=sys.stderr)
        for item in missing:
            print(f"- {item}", file=sys.stderr)
        return 1
    print("Required package assets are present.")
    return 0


def _single_archive(dist_path: Path, pattern: str) -> Path | None:
    matches = sorted(dist_path.glob(pattern))
    if len(matches) != 1:
        return None
    return matches[0]


def _required_assets_from_source() -> tuple[str, ...]:
    package_assets = Path("src/rail/package_assets")
    if not package_assets.is_dir():
        return REQUIRED_PACKAGE_ASSETS
    return tuple(
        str(Path("package_assets") / path.relative_to(package_assets))
        for path in sorted(package_assets.rglob("*"))
        if path.is_file()
    )


def _wheel_members(path: Path) -> set[str]:
    with zipfile.ZipFile(path) as archive:
        return set(archive.namelist())


def _sdist_members(path: Path) -> set[str]:
    with tarfile.open(path, "r:gz") as archive:
        return set(archive.getnames())


def _sdist_contains(members: set[str], expected: str) -> bool:
    return expected in members or any(member.endswith(f"/{expected}") for member in members)


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
