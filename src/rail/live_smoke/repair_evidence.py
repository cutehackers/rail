from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from pydantic import BaseModel, ConfigDict

from rail.auth.redaction import redact_secrets
from rail.live_smoke.models import LiveSmokeActor, LiveSmokeReport, OwningSurface, SymptomClass


class RepairEvidenceSummary(BaseModel):
    model_config = ConfigDict(extra="forbid")

    actor: LiveSmokeActor
    report_path: Path
    symptom_class: SymptomClass
    owning_surface: OwningSurface
    error_text: str
    policy_violation_reason: str | None
    output_schema_ref: str | None
    output_schema_digest: str | None
    evidence_refs: list[str]
    seed_digest: str | None
    fixture_digest: str


def summarize_repair_evidence(report_path: Path) -> RepairEvidenceSummary:
    report_path = report_path.resolve(strict=True)
    report = LiveSmokeReport.model_validate_json(report_path.read_text(encoding="utf-8"))
    runtime_evidence = _load_runtime_evidence(report)
    redacted_evidence = redact_secrets(runtime_evidence)

    return RepairEvidenceSummary(
        actor=report.actor,
        report_path=report_path,
        symptom_class=report.symptom_class or SymptomClass.UNKNOWN_FAILURE,
        owning_surface=report.owning_surface or OwningSurface.UNKNOWN,
        error_text=_optional_string(redacted_evidence.get("error")) or "",
        policy_violation_reason=_policy_violation_reason(redacted_evidence),
        output_schema_ref=_optional_string(redacted_evidence.get("output_schema_ref")),
        output_schema_digest=_optional_string(redacted_evidence.get("output_schema_digest")),
        evidence_refs=list(report.evidence_refs),
        seed_digest=report.seed_digest,
        fixture_digest=report.fixture_digest,
    )


def _load_runtime_evidence(report: LiveSmokeReport) -> dict[str, Any]:
    if report.artifact_dir is None:
        return {}
    for evidence_ref in report.evidence_refs:
        if not evidence_ref.endswith(".runtime_evidence.json"):
            continue
        evidence_path = report.artifact_dir / evidence_ref
        if not evidence_path.is_file():
            return {}
        try:
            loaded = json.loads(evidence_path.read_text(encoding="utf-8"))
        except json.JSONDecodeError:
            return {}
        return loaded if isinstance(loaded, dict) else {}
    return {}


def _policy_violation_reason(evidence: dict[str, Any]) -> str | None:
    policy_violation = evidence.get("policy_violation")
    if isinstance(policy_violation, dict):
        reason = _optional_string(policy_violation.get("reason"))
        if reason is not None:
            return reason
    return None


def _optional_string(value: object) -> str | None:
    return value if isinstance(value, str) else None
