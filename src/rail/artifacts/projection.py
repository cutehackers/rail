from __future__ import annotations

import json
from pathlib import Path

import yaml
from pydantic import BaseModel, ConfigDict

from rail.artifacts.models import ArtifactHandle
from rail.artifacts.store import validate_artifact_handle
from rail.artifacts.terminal_summary import project_terminal_summary


class StatusProjection(BaseModel):
    model_config = ConfigDict(extra="forbid")

    current_phase: str
    current_actor: str | None
    interruption: str | None = None
    terminal_state: str | None = None
    next_step: str


class ResultProjection(BaseModel):
    model_config = ConfigDict(extra="forbid")

    outcome: str
    current_phase: str
    terminal_decision: str | None
    evidence_refs: list[str]
    changed_files: list[str]
    residual_risk: str
    next_step: str


def project_status(handle: ArtifactHandle) -> StatusProjection:
    handle = validate_artifact_handle(handle)
    run_status = _load_run_status(handle)
    raw_terminal = run_status.get("outcome")
    terminal = raw_terminal if isinstance(raw_terminal, str) else None
    raw_actor = run_status.get("current_actor")
    current_actor = raw_actor if isinstance(raw_actor, str) else None
    return StatusProjection(
        current_phase="terminal" if run_status.get("status") == "terminal" else "running",
        current_actor=current_actor,
        terminal_state=terminal,
        next_step="complete" if terminal == "pass" else "inspect",
    )


def project_result(handle: ArtifactHandle) -> ResultProjection:
    handle = validate_artifact_handle(handle)
    status = project_status(handle)
    summary = project_terminal_summary(handle)
    evidence_refs = sorted(path.relative_to(handle.artifact_dir).as_posix() for path in (handle.artifact_dir / "runs").glob("*"))
    return ResultProjection(
        outcome=summary.outcome,
        current_phase=status.current_phase,
        terminal_decision=status.terminal_state,
        evidence_refs=evidence_refs,
        changed_files=_changed_files(handle.artifact_dir),
        residual_risk="low" if summary.outcome == "pass" else "medium",
        next_step=summary.next_step,
    )


def _load_run_status(handle: ArtifactHandle) -> dict[str, object]:
    return yaml.safe_load((handle.artifact_dir / "run_status.yaml").read_text(encoding="utf-8")) or {}


def _changed_files(artifact_dir: Path) -> list[str]:
    generator_evidence = artifact_dir / "runs" / "generator.runtime_evidence.json"
    if not generator_evidence.is_file():
        return []
    payload = json.loads(generator_evidence.read_text(encoding="utf-8"))
    structured = payload.get("structured_output", {})
    if isinstance(structured, dict):
        changed_files = structured.get("changed_files", [])
        if isinstance(changed_files, list):
            return [str(path) for path in changed_files]
    return []
