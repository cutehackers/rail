from __future__ import annotations

from pathlib import Path
from typing import Literal

import yaml
from pydantic import BaseModel, ConfigDict

from rail.artifacts.models import ArtifactHandle
from rail.artifacts.store import validate_artifact_handle
from rail.auth.redaction import redact_secrets

BlockedCategory = Literal["runtime", "validation", "policy", "environment"] | None


class TerminalSummaryProjection(BaseModel):
    model_config = ConfigDict(extra="forbid")

    artifact_id: str
    outcome: str
    reason: str
    blocked_category: BlockedCategory = None
    evidence_refs: list[str]
    next_step: str


def project_terminal_summary(handle: ArtifactHandle) -> TerminalSummaryProjection:
    handle = validate_artifact_handle(handle)
    run_status = _load_run_status(handle)
    outcome = _status_value(run_status.get("outcome"), default="unknown")
    blocked_category = _blocked_category(run_status.get("blocked_category")) if outcome == "blocked" else None
    reason = str(redact_secrets(_status_value(run_status.get("reason"), default=_default_reason(outcome, blocked_category))))

    return TerminalSummaryProjection(
        artifact_id=handle.artifact_id,
        outcome=outcome,
        reason=reason,
        blocked_category=blocked_category,
        evidence_refs=_evidence_refs(handle.artifact_dir, _attempt_ref(run_status)),
        next_step=_next_step(outcome, blocked_category),
    )


def write_terminal_summary(handle: ArtifactHandle) -> TerminalSummaryProjection:
    summary = project_terminal_summary(handle)
    (handle.artifact_dir / "terminal_summary.yaml").write_text(
        yaml.safe_dump(summary.model_dump(mode="json"), sort_keys=True),
        encoding="utf-8",
    )
    return summary


def _load_run_status(handle: ArtifactHandle) -> dict[str, object]:
    status_path = handle.artifact_dir / "run_status.yaml"
    if not status_path.is_file():
        return {}
    payload = yaml.safe_load(status_path.read_text(encoding="utf-8")) or {}
    if not isinstance(payload, dict):
        return {}
    return payload


def _evidence_refs(artifact_dir: Path, attempt_ref: str | None) -> list[str]:
    runs_dir = artifact_dir / "runs" / attempt_ref if attempt_ref else artifact_dir / "runs"
    refs = [
        str(redact_secrets(path.relative_to(artifact_dir).as_posix()))
        for path in runs_dir.glob("*")
        if path.is_file()
    ]
    validation_ref = artifact_dir / "validation" / "evidence.yaml"
    if validation_ref.is_file():
        refs.append(str(redact_secrets(validation_ref.relative_to(artifact_dir).as_posix())))
    return sorted(refs)


def _attempt_ref(run_status: dict[str, object]) -> str | None:
    value = run_status.get("attempt_ref")
    return value if isinstance(value, str) and value else None


def _status_value(value: object, *, default: str) -> str:
    return value if isinstance(value, str) and value else default


def _blocked_category(value: object) -> BlockedCategory:
    if value in {"runtime", "validation", "policy", "environment"}:
        return value  # type: ignore[return-value]
    return "runtime"


def _default_reason(outcome: str, blocked_category: BlockedCategory) -> str:
    if outcome == "pass":
        return "Rail accepted the result after validation and evaluator gate checks."
    if outcome == "reject":
        return "The evaluator rejected the result."
    if outcome == "blocked":
        category = blocked_category or "runtime"
        return f"Rail blocked the workflow in the {category} stage."
    return "Rail has no terminal decision for this artifact yet."


def _next_step(outcome: str, blocked_category: BlockedCategory) -> str:
    if outcome == "pass":
        return "complete"
    if outcome == "reject":
        return "inspect evaluator findings and revise the task"
    if blocked_category == "runtime":
        return "fix Actor Runtime readiness or inspect runtime evidence"
    if blocked_category == "validation":
        return "inspect validation logs and rerun supervision after fixing failures"
    if blocked_category == "policy":
        return "inspect policy and patch evidence before retrying"
    if blocked_category == "environment":
        return "fix environment readiness and retry supervision"
    return "inspect artifact evidence"
