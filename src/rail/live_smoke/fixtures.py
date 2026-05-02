from __future__ import annotations

import shutil
from dataclasses import dataclass
from importlib.resources import as_file, files
from importlib.resources.abc import Traversable
from pathlib import Path

from rail.workspace.isolation import tree_digest


@dataclass(frozen=True)
class CopiedFixtureTarget:
    target_root: Path
    fixture_digest: str


def live_smoke_fixture_source() -> Path:
    fixture_resource = _live_smoke_fixture_resource()
    if not isinstance(fixture_resource, Path):
        raise RuntimeError("live smoke fixture target must be available on the filesystem")
    return fixture_resource


def copy_fixture_target(target_root: Path, *, report_root: Path) -> CopiedFixtureTarget:
    with as_file(_live_smoke_fixture_resource()) as fixture_source:
        _validate_target_outside_source(target_root, fixture_source)
        if target_root.exists():
            shutil.rmtree(target_root)
        shutil.copytree(fixture_source, target_root)
    return CopiedFixtureTarget(target_root=target_root, fixture_digest=tree_digest(target_root))


def _live_smoke_fixture_resource() -> Traversable:
    return files("rail").joinpath("package_assets", "live_smoke", "fixture_target")


def _validate_target_outside_source(target_root: Path, fixture_source: Path) -> None:
    resolved_target = target_root.resolve(strict=False)
    resolved_source = fixture_source.resolve(strict=True)
    if (
        resolved_target == resolved_source
        or _is_relative_to(resolved_target, resolved_source)
        or _is_relative_to(resolved_source, resolved_target)
    ):
        raise ValueError("fixture target must not overlap the packaged fixture source")


def _is_relative_to(path: Path, parent: Path) -> bool:
    try:
        path.relative_to(parent)
    except ValueError:
        return False
    return True
