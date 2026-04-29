from __future__ import annotations

from pathlib import Path
from typing import Literal

from pydantic import BaseModel, ConfigDict

from rail.workspace.sandbox import Sandbox


class PatchOperation(BaseModel):
    model_config = ConfigDict(extra="forbid")

    op: Literal["write"] = "write"
    path: str
    content: str
    executable: bool = False
    binary: bool = False


class PatchBundle(BaseModel):
    model_config = ConfigDict(extra="forbid")

    schema_version: Literal["1"] = "1"
    base_tree_digest: str
    operations: list[PatchOperation]
    max_files: int = 100
    max_bytes: int = 1_000_000
    allow_binary: bool = False
    allow_executable: bool = False
    allow_artifact_writes: bool = False


def build_patch_bundle(sandbox: Sandbox, relative_paths: list[str]) -> PatchBundle:
    operations = [
        PatchOperation(path=relative_path, content=(sandbox.sandbox_root / relative_path).read_text(encoding="utf-8"))
        for relative_path in relative_paths
    ]
    bundle = PatchBundle(base_tree_digest=sandbox.base_tree_digest, operations=operations)
    validate_patch_bundle(bundle)
    return bundle


def validate_patch_bundle(bundle: PatchBundle) -> PatchBundle:
    if len(bundle.operations) > bundle.max_files:
        raise ValueError("patch bundle file count exceeds policy")
    total_size = 0
    for operation in bundle.operations:
        _validate_path(operation.path, allow_artifact_writes=bundle.allow_artifact_writes)
        total_size += len(operation.content.encode("utf-8"))
        if operation.binary and not bundle.allow_binary:
            raise ValueError("binary patch operations require explicit policy")
        if operation.executable and not bundle.allow_executable:
            raise ValueError("executable mode changes require explicit policy")
    if total_size > bundle.max_bytes:
        raise ValueError("patch bundle size exceeds policy")
    return bundle


def _validate_path(path_value: str, *, allow_artifact_writes: bool) -> None:
    path = Path(path_value)
    if path.is_absolute() or ".." in path.parts:
        raise ValueError("patch paths must be relative and cannot traverse")
    if path.parts[:2] == (".harness", "artifacts") and not allow_artifact_writes:
        raise ValueError("writes to .harness/artifacts require explicit evidence policy")
