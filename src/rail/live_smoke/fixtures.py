from __future__ import annotations

import shutil
from dataclasses import dataclass
from importlib.resources import files
from pathlib import Path

from rail.workspace.isolation import tree_digest


@dataclass(frozen=True)
class CopiedFixtureTarget:
    target_root: Path
    fixture_digest: str


def live_smoke_fixture_source() -> Path:
    fixture_source = files("rail").joinpath("package_assets", "live_smoke", "fixture_target")
    if not isinstance(fixture_source, Path):
        raise RuntimeError("live smoke fixture target must be available on the filesystem")
    return fixture_source


def copy_fixture_target(target_root: Path, *, report_root: Path) -> CopiedFixtureTarget:
    shutil.copytree(
        live_smoke_fixture_source(),
        target_root,
        ignore=shutil.ignore_patterns(report_root.name),
    )
    return CopiedFixtureTarget(target_root=target_root, fixture_digest=tree_digest(target_root))
