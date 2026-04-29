from __future__ import annotations

from pathlib import Path

from rail.workspace.isolation import tree_digest
from rail.workspace.patch_bundle import PatchBundle, validate_patch_bundle


def apply_patch_bundle(bundle: PatchBundle, target_root: Path) -> None:
    target_root = target_root.resolve(strict=True)
    validate_patch_bundle(bundle)
    if tree_digest(target_root) != bundle.base_tree_digest:
        raise ValueError("target base tree digest does not match patch bundle")

    for operation in bundle.operations:
        if tree_digest(target_root) != bundle.base_tree_digest:
            raise ValueError("target base tree changed during patch apply")
        target_path = (target_root / operation.path).resolve(strict=False)
        if not target_path.is_relative_to(target_root):
            raise ValueError("patch operation escapes target root")
        target_path.parent.mkdir(parents=True, exist_ok=True)
        target_path.write_text(operation.content, encoding="utf-8")
