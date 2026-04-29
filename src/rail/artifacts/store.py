from __future__ import annotations

import hashlib
import json
from datetime import datetime, timezone
from pathlib import Path
from uuid import uuid4

import yaml
from pydantic import ValidationError

from rail.artifacts.models import ArtifactHandle, RunStatus, TerminalSummary, WorkflowState
from rail.request import HarnessRequest, normalize_draft

_REQUEST_SNAPSHOT = "request.yaml"


class ArtifactStore:
    def __init__(self, project_root: Path) -> None:
        self.project_root = _canonical_project_root(project_root)
        self.artifacts_root = self.project_root / ".harness" / "artifacts"

    @classmethod
    def for_project(cls, project_root: str | Path) -> ArtifactStore:
        return cls(Path(project_root))

    def allocate(self, draft: HarnessRequest) -> ArtifactHandle:
        request = normalize_draft(draft)
        self.artifacts_root.mkdir(parents=True, exist_ok=True)
        artifact_id, artifact_dir = self._allocate_artifact_dir()
        request_snapshot_digest = digest_request(request)
        handle = ArtifactHandle(
            artifact_id=artifact_id,
            artifact_dir=artifact_dir.resolve(strict=True),
            project_root=self.project_root,
            request_snapshot_digest=request_snapshot_digest,
            created_at=datetime.now(timezone.utc),
        )

        _write_yaml(artifact_dir / _REQUEST_SNAPSHOT, request.model_dump(mode="json", by_alias=True))
        _write_yaml(artifact_dir / "state.yaml", WorkflowState(artifact_id=artifact_id).model_dump(mode="json"))
        _write_yaml(artifact_dir / "workflow.yaml", _workflow_payload(artifact_id, request))
        _write_yaml(artifact_dir / "run_status.yaml", RunStatus(artifact_id=artifact_id).model_dump(mode="json"))
        _write_yaml(artifact_dir / "terminal_summary.yaml", TerminalSummary(artifact_id=artifact_id).model_dump(mode="json"))
        (artifact_dir / "runs").mkdir()

        return validate_artifact_handle(handle)

    def _allocate_artifact_dir(self) -> tuple[str, Path]:
        for _ in range(100):
            artifact_id = f"rail-{uuid4().hex}"
            artifact_dir = self.artifacts_root / artifact_id
            try:
                artifact_dir.mkdir()
            except FileExistsError:
                continue
            return artifact_id, artifact_dir
        raise RuntimeError("could not allocate a unique artifact directory")


def validate_artifact_handle(handle: ArtifactHandle) -> ArtifactHandle:
    project_root = _canonical_project_root(handle.project_root)
    artifact_dir_input = Path(handle.artifact_dir)
    if artifact_dir_input.is_symlink():
        raise ValueError("artifact_dir must not be a symlink")
    if not artifact_dir_input.exists():
        raise ValueError("artifact_dir does not exist")

    artifact_dir = artifact_dir_input.resolve(strict=True)
    artifact_owner = _artifact_owner_project_root(artifact_dir)
    if artifact_owner is not None and artifact_owner != project_root:
        raise ValueError("project_root does not match artifact_dir")

    artifacts_root = (project_root / ".harness" / "artifacts").resolve(strict=False)
    if not _is_relative_to(artifact_dir, artifacts_root):
        raise ValueError("artifact_dir must be inside the project artifact store")

    request_snapshot = artifact_dir / _REQUEST_SNAPSHOT
    if not request_snapshot.is_file():
        raise ValueError("request snapshot is missing")

    try:
        request = HarnessRequest.model_validate(yaml.safe_load(request_snapshot.read_text(encoding="utf-8")))
    except (ValidationError, yaml.YAMLError) as exc:
        raise ValueError("request snapshot digest mismatch") from exc

    actual_digest = digest_request(request)
    if actual_digest != handle.request_snapshot_digest:
        raise ValueError("request snapshot digest mismatch")

    return handle.model_copy(update={"artifact_dir": artifact_dir, "project_root": project_root})


def digest_request(request: HarnessRequest) -> str:
    payload = json.dumps(request.model_dump(mode="json", by_alias=True), sort_keys=True, separators=(",", ":"))
    return "sha256:" + hashlib.sha256(payload.encode("utf-8")).hexdigest()


def _canonical_project_root(project_root: str | Path) -> Path:
    path = Path(project_root)
    if path.is_symlink():
        raise ValueError("project_root must not be a symlink")
    if not path.exists():
        raise ValueError("project_root does not exist")
    return path.resolve(strict=True)


def _artifact_owner_project_root(artifact_dir: Path) -> Path | None:
    if artifact_dir.parent.name != "artifacts":
        return None
    if artifact_dir.parent.parent.name != ".harness":
        return None
    return artifact_dir.parent.parent.parent.resolve(strict=True)


def _is_relative_to(path: Path, parent: Path) -> bool:
    try:
        path.relative_to(parent)
    except ValueError:
        return False
    return True


def _write_yaml(path: Path, payload: object) -> None:
    path.write_text(yaml.safe_dump(payload, sort_keys=True), encoding="utf-8")


def _workflow_payload(artifact_id: str, request: HarnessRequest) -> dict[str, object]:
    return {
        "schema_version": "1",
        "artifact_id": artifact_id,
        "task_type": request.task_type,
        "status": "created",
    }
