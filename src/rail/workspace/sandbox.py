from __future__ import annotations

import shutil
import tempfile
from pathlib import Path

from pydantic import BaseModel, ConfigDict

from rail.workspace.isolation import is_hardlink, tree_digest


class Sandbox(BaseModel):
    model_config = ConfigDict(extra="forbid")

    target_root: Path
    sandbox_root: Path
    base_tree_digest: str


def create_sandbox(target_root: Path) -> Sandbox:
    target_root = target_root.resolve(strict=True)
    sandbox_root = Path(tempfile.mkdtemp(prefix="rail-sandbox-")).resolve(strict=True)
    shutil.copytree(target_root, sandbox_root, dirs_exist_ok=True, ignore=shutil.ignore_patterns(".git", ".harness"))
    return Sandbox(target_root=target_root, sandbox_root=sandbox_root, base_tree_digest=tree_digest(target_root))


def write_sandbox_file(sandbox: Sandbox, relative_path: str, content: str) -> Path:
    path = _safe_sandbox_path(sandbox, relative_path)
    if path.is_symlink():
        raise ValueError("sandbox writes through symlinks are rejected")
    if is_hardlink(path):
        raise ValueError("sandbox writes through hardlinks are rejected")
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    return path


def _safe_sandbox_path(sandbox: Sandbox, relative_path: str) -> Path:
    candidate = Path(relative_path)
    if candidate.is_absolute() or ".." in candidate.parts:
        raise ValueError("sandbox paths must be relative and stay inside the sandbox")
    unresolved = sandbox.sandbox_root / candidate
    if unresolved.is_symlink():
        raise ValueError("sandbox writes through symlinks are rejected")
    path = unresolved.resolve(strict=False)
    if not path.is_relative_to(sandbox.sandbox_root):
        raise ValueError("sandbox path escapes sandbox root")
    return path
