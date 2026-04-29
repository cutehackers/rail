from __future__ import annotations

from pathlib import Path

from rail.workspace.isolation import tree_digest
from rail.workspace.patch_bundle import PatchBundle, PatchValidationPolicy, validate_patch_bundle


def apply_patch_bundle(bundle: PatchBundle, target_root: Path, *, policy: PatchValidationPolicy | None = None) -> None:
    target_root = target_root.resolve(strict=True)
    validate_patch_bundle(bundle, policy=policy)
    if tree_digest(target_root) != bundle.base_tree_digest:
        raise ValueError("target base tree digest does not match patch bundle")

    prepared: list[tuple[Path, str | None, str]] = []
    for operation in bundle.operations:
        target_path = (target_root / operation.path).resolve(strict=False)
        if not target_path.is_relative_to(target_root):
            raise ValueError("patch operation escapes target root")
        if target_path.is_symlink():
            raise ValueError("patch operation targets a symlink")
        old_content = target_path.read_text(encoding="utf-8") if target_path.exists() else None
        prepared.append((target_path, old_content, operation.content))

    if tree_digest(target_root) != bundle.base_tree_digest:
        raise ValueError("target base tree changed during patch apply")

    applied: list[tuple[Path, str | None]] = []
    try:
        for target_path, old_content, new_content in prepared:
            target_path.parent.mkdir(parents=True, exist_ok=True)
            target_path.write_text(new_content, encoding="utf-8")
            applied.append((target_path, old_content))
    except Exception:
        for target_path, old_content in reversed(applied):
            if old_content is None:
                target_path.unlink(missing_ok=True)
            else:
                target_path.write_text(old_content, encoding="utf-8")
        raise
