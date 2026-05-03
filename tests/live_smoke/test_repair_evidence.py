from __future__ import annotations

import json
from pathlib import Path

import pytest
from pydantic import ValidationError

from rail.live_smoke.models import LiveSmokeActor, LiveSmokeReport, LiveSmokeVerdict, OwningSurface, SymptomClass
from rail.live_smoke.repair_evidence import RepairEvidenceSummary, summarize_repair_evidence


def _failed_report(tmp_path: Path, *, error: str = "shell executable is not allowed: python") -> Path:
    artifact_dir = tmp_path / ".harness" / "artifacts" / "rail-abc"
    evidence_ref = Path("runs/attempt-0001/generator.runtime_evidence.json")
    evidence_path = artifact_dir / evidence_ref
    evidence_path.parent.mkdir(parents=True)
    evidence_path.write_text(
        json.dumps(
            {
                "actor": "generator",
                "error": error,
                "policy_violation": {"reason": "shell executable is not allowed: python"},
                "output_schema_ref": "actor_runtime/schemas/generator.schema.json",
                "output_schema_digest": "sha256:schema",
            }
        ),
        encoding="utf-8",
    )
    report = LiveSmokeReport(
        actor=LiveSmokeActor.GENERATOR,
        verdict=LiveSmokeVerdict.FAILED,
        symptom_class=SymptomClass.POLICY_VIOLATION,
        owning_surface=OwningSurface.RUNTIME_CONTRACT,
        artifact_id="rail-abc",
        artifact_dir=artifact_dir,
        report_dir=tmp_path / "reports" / "generator",
        fixture_digest="sha256:fixture",
        seed_schema_version="1",
        seed_digest="sha256:seed",
        synthetic_seed=True,
        evidence_refs=[evidence_ref.as_posix()],
        repair_proposal=None,
    )
    report_path = report.report_dir / "live_smoke_report.json"
    report_path.parent.mkdir(parents=True)
    report_path.write_text(report.model_dump_json(indent=2), encoding="utf-8")
    return report_path


def test_summarize_repair_evidence_extracts_relevant_fields(tmp_path: Path) -> None:
    report_path = _failed_report(tmp_path)

    summary = summarize_repair_evidence(report_path)

    assert summary.actor == LiveSmokeActor.GENERATOR
    assert summary.report_path == report_path
    assert summary.symptom_class == SymptomClass.POLICY_VIOLATION
    assert summary.owning_surface == OwningSurface.RUNTIME_CONTRACT
    assert summary.error_text == "shell executable is not allowed: python"
    assert summary.policy_violation_reason == "shell executable is not allowed: python"
    assert summary.output_schema_ref == "actor_runtime/schemas/generator.schema.json"
    assert summary.output_schema_digest == "sha256:schema"
    assert summary.evidence_refs == ["runs/attempt-0001/generator.runtime_evidence.json"]
    assert summary.seed_digest == "sha256:seed"
    assert summary.fixture_digest == "sha256:fixture"


def test_summarize_repair_evidence_redacts_secret_shaped_values(tmp_path: Path) -> None:
    report_path = _failed_report(
        tmp_path,
        error="validation failed with OPENAI_API_KEY=sk-test-secret and sk-live-secret",
    )

    summary = summarize_repair_evidence(report_path)

    assert "sk-test-secret" not in summary.error_text
    assert "sk-live-secret" not in summary.error_text
    assert "OPENAI_API_KEY=[REDACTED]" in summary.error_text


def test_summarize_repair_evidence_handles_missing_runtime_evidence(tmp_path: Path) -> None:
    report = LiveSmokeReport(
        actor=LiveSmokeActor.GENERATOR,
        verdict=LiveSmokeVerdict.FAILED,
        symptom_class=SymptomClass.EVIDENCE_WRITER_FAILURE,
        owning_surface=OwningSurface.PROVIDER,
        artifact_id="rail-abc",
        artifact_dir=tmp_path / ".harness" / "artifacts" / "rail-abc",
        report_dir=tmp_path / "reports" / "generator",
        fixture_digest="sha256:fixture",
        evidence_refs=["runs/attempt-0001/generator.runtime_evidence.json"],
        repair_proposal=None,
    )
    report_path = report.report_dir / "live_smoke_report.json"
    report_path.parent.mkdir(parents=True)
    report_path.write_text(report.model_dump_json(indent=2), encoding="utf-8")

    summary = summarize_repair_evidence(report_path)

    assert summary.error_text == ""
    assert summary.policy_violation_reason is None
    assert summary.output_schema_ref is None


def test_repair_evidence_summary_rejects_unknown_fields() -> None:
    with pytest.raises(ValidationError):
        RepairEvidenceSummary(
            actor=LiveSmokeActor.GENERATOR,
            report_path=Path("live_smoke_report.json"),
            symptom_class=SymptomClass.POLICY_VIOLATION,
            owning_surface=OwningSurface.RUNTIME_CONTRACT,
            error_text="",
            policy_violation_reason=None,
            output_schema_ref=None,
            output_schema_digest=None,
            evidence_refs=[],
            seed_digest=None,
            fixture_digest="sha256:fixture",
            unexpected=True,
        )
