from __future__ import annotations

from pathlib import Path

import yaml

from rail.artifacts.models import ArtifactHandle
from rail.artifacts.store import validate_artifact_handle


def write_handle_file(handle: ArtifactHandle) -> Path:
    path = handle.artifact_dir / "handle.yaml"
    path.write_text(yaml.safe_dump(handle.model_dump(mode="json"), sort_keys=True), encoding="utf-8")
    return path


def load_handle_file(path: str | Path) -> ArtifactHandle:
    handle_path = Path(path)
    if handle_path.is_symlink():
        raise ValueError("handle path must not be a symlink")
    payload = yaml.safe_load(handle_path.read_text(encoding="utf-8"))
    handle = ArtifactHandle.model_validate(payload)
    return validate_artifact_handle(handle)
