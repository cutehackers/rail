from __future__ import annotations

import hashlib
import os
from pathlib import Path

_ALLOWED_ENV_KEYS = {"OPENAI_API_KEY", "RAIL_ARTIFACT", "RAIL_RUN_ID"}


def tree_digest(root: Path) -> str:
    digest = hashlib.sha256()
    for path in sorted(root.rglob("*")):
        if not path.is_file() or ".git" in path.parts or ".harness" in path.parts:
            continue
        relative = path.relative_to(root).as_posix()
        stat = path.stat()
        digest.update(relative.encode("utf-8"))
        digest.update(str(stat.st_mode & 0o777).encode("utf-8"))
        digest.update(path.read_bytes())
    return "sha256:" + digest.hexdigest()


def assert_target_unchanged(before: str, after: str) -> None:
    if before != after:
        raise ValueError("target tree changed outside Rail patch apply")


def deny_target_path_input(value: str, target_root: Path) -> None:
    path = Path(value)
    if path.is_absolute() and _is_relative_to(path.resolve(strict=False), target_root.resolve(strict=False)):
        raise ValueError("absolute target path inputs are denied")


def scrub_actor_environment(env: dict[str, str]) -> dict[str, str]:
    return {key: value for key, value in env.items() if key in _ALLOWED_ENV_KEYS}


def is_hardlink(path: Path) -> bool:
    return path.exists() and os.stat(path).st_nlink > 1


def _is_relative_to(path: Path, parent: Path) -> bool:
    try:
        path.relative_to(parent)
    except ValueError:
        return False
    return True
