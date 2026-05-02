from __future__ import annotations

from pathlib import Path

import pytest
from pydantic import ValidationError

from rail.live_smoke.contracts import V1_LIVE_SMOKE_ACTORS, evaluate_behavior_smoke
from rail.live_smoke.models import (
    LiveSmokeActor,
    LiveSmokeReport,
    LiveSmokeVerdict,
    OwningSurface,
    RepairProposal,
    SymptomClass,
)


def test_live_smoke_report_rejects_unknown_fields(tmp_path: Path) -> None:
    with pytest.raises(ValidationError):
        LiveSmokeReport(
            actor=LiveSmokeActor.PLANNER,
            verdict=LiveSmokeVerdict.PASSED,
            symptom_class=None,
            owning_surface=None,
            report_dir=tmp_path,
            fixture_digest="sha256:abc",
            evidence_refs=[],
            repair_proposal=None,
            unexpected=True,
        )


@pytest.mark.parametrize(
    ("field_name", "value"),
    [
        ("symptom_class", SymptomClass.BEHAVIOR_SMOKE_FAILURE),
        ("owning_surface", OwningSurface.ACTOR_PROMPT),
        (
            "repair_proposal",
            RepairProposal(
                owning_surface=OwningSurface.ACTOR_PROMPT,
                file_paths=[".harness/actors/context_builder.md"],
                summary="Forbid grep fallback in context collection.",
                preserves_fail_closed_policy=True,
            ),
        ),
    ],
)
def test_passed_live_smoke_report_rejects_failure_details(
    tmp_path: Path,
    field_name: str,
    value: object,
) -> None:
    report_data = {
        "actor": LiveSmokeActor.PLANNER,
        "verdict": LiveSmokeVerdict.PASSED,
        "symptom_class": None,
        "owning_surface": None,
        "report_dir": tmp_path,
        "fixture_digest": "sha256:abc",
        "evidence_refs": [],
        "repair_proposal": None,
    }
    report_data[field_name] = value

    with pytest.raises(ValidationError):
        LiveSmokeReport(**report_data)


def test_passed_live_smoke_report_accepts_success_payload(tmp_path: Path) -> None:
    report = LiveSmokeReport(
        actor=LiveSmokeActor.PLANNER,
        verdict=LiveSmokeVerdict.PASSED,
        symptom_class=None,
        owning_surface=None,
        report_dir=tmp_path,
        fixture_digest="sha256:abc",
        evidence_refs=[],
        repair_proposal=None,
    )

    assert report.verdict == LiveSmokeVerdict.PASSED


def test_failed_live_smoke_report_accepts_failure_classification(
    tmp_path: Path,
) -> None:
    proposal = RepairProposal(
        owning_surface=OwningSurface.ACTOR_PROMPT,
        file_paths=[".harness/actors/context_builder.md"],
        summary="Forbid grep fallback in context collection.",
        preserves_fail_closed_policy=True,
    )

    report = LiveSmokeReport(
        actor=LiveSmokeActor.PLANNER,
        verdict=LiveSmokeVerdict.FAILED,
        symptom_class=SymptomClass.BEHAVIOR_SMOKE_FAILURE,
        owning_surface=OwningSurface.ACTOR_PROMPT,
        report_dir=tmp_path,
        fixture_digest="sha256:abc",
        evidence_refs=[],
        repair_proposal=proposal,
    )

    assert report.symptom_class == SymptomClass.BEHAVIOR_SMOKE_FAILURE


@pytest.mark.parametrize("field_name", ["symptom_class", "owning_surface"])
def test_failed_live_smoke_report_requires_failure_classification(
    tmp_path: Path,
    field_name: str,
) -> None:
    report_data = {
        "actor": LiveSmokeActor.CONTEXT_BUILDER,
        "verdict": LiveSmokeVerdict.FAILED,
        "symptom_class": SymptomClass.BEHAVIOR_SMOKE_FAILURE,
        "owning_surface": OwningSurface.ACTOR_PROMPT,
        "report_dir": tmp_path,
        "fixture_digest": "sha256:abc",
        "evidence_refs": [],
        "repair_proposal": None,
    }
    report_data[field_name] = None

    with pytest.raises(ValidationError):
        LiveSmokeReport(**report_data)


def test_failed_live_smoke_report_accepts_repair_proposal(tmp_path: Path) -> None:
    proposal = RepairProposal(
        owning_surface=OwningSurface.RUNTIME_CONTRACT,
        file_paths=["src/rail/actor_runtime/schemas.py"],
        summary="Keep live smoke actor output aligned with strict schemas.",
        preserves_fail_closed_policy=True,
    )

    report = LiveSmokeReport(
        actor=LiveSmokeActor.CONTEXT_BUILDER,
        verdict=LiveSmokeVerdict.FAILED,
        symptom_class=SymptomClass.SCHEMA_MISMATCH,
        owning_surface=OwningSurface.RUNTIME_CONTRACT,
        report_dir=tmp_path,
        fixture_digest="sha256:abc",
        evidence_refs=[],
        repair_proposal=proposal,
    )

    assert report.repair_proposal == proposal


def test_failed_live_smoke_report_rejects_repair_proposal_for_nonrepairable_surface(
    tmp_path: Path,
) -> None:
    proposal = RepairProposal(
        owning_surface=OwningSurface.RUNTIME_INVOCATION,
        file_paths=["src/rail/actor_runtime/runtime.py"],
        summary="Keep runtime invocation compatible with live smoke.",
        preserves_fail_closed_policy=True,
    )

    with pytest.raises(ValidationError):
        LiveSmokeReport(
            actor=LiveSmokeActor.PLANNER,
            verdict=LiveSmokeVerdict.FAILED,
            symptom_class=SymptomClass.PROVIDER_TRANSIENT_FAILURE,
            owning_surface=OwningSurface.PROVIDER,
            report_dir=tmp_path,
            fixture_digest="sha256:abc",
            evidence_refs=[],
            repair_proposal=proposal,
        )


def test_failed_live_smoke_report_rejects_mismatched_repair_proposal_surface(
    tmp_path: Path,
) -> None:
    proposal = RepairProposal(
        owning_surface=OwningSurface.RUNTIME_CONTRACT,
        file_paths=["src/rail/actor_runtime/schemas.py"],
        summary="Keep live smoke actor output aligned with strict schemas.",
        preserves_fail_closed_policy=True,
    )

    with pytest.raises(ValidationError):
        LiveSmokeReport(
            actor=LiveSmokeActor.CONTEXT_BUILDER,
            verdict=LiveSmokeVerdict.FAILED,
            symptom_class=SymptomClass.BEHAVIOR_SMOKE_FAILURE,
            owning_surface=OwningSurface.ACTOR_PROMPT,
            report_dir=tmp_path,
            fixture_digest="sha256:abc",
            evidence_refs=[],
            repair_proposal=proposal,
        )


def test_repair_proposal_records_safe_rail_owned_surface() -> None:
    proposal = RepairProposal(
        owning_surface=OwningSurface.ACTOR_PROMPT,
        file_paths=[".harness/actors/context_builder.md"],
        summary="Forbid grep fallback in context collection.",
        preserves_fail_closed_policy=True,
    )

    assert proposal.owning_surface == OwningSurface.ACTOR_PROMPT
    assert proposal.preserves_fail_closed_policy is True


def test_v1_live_smoke_actor_scope_is_planner_and_context_builder_only() -> None:
    assert V1_LIVE_SMOKE_ACTORS == (
        LiveSmokeActor.PLANNER,
        LiveSmokeActor.CONTEXT_BUILDER,
    )


def test_planner_behavior_smoke_requires_minimum_fields() -> None:
    error = evaluate_behavior_smoke(
        LiveSmokeActor.PLANNER,
        {"summary": "Plan", "substeps": [], "risks": [], "acceptance_criteria_refined": []},
    )

    assert error is None


def test_context_builder_behavior_smoke_requires_non_empty_context() -> None:
    error = evaluate_behavior_smoke(
        LiveSmokeActor.CONTEXT_BUILDER,
        {
            "relevant_files": [{"path": "README.md", "why": "entry point"}],
            "repo_patterns": ["small service module"],
            "test_patterns": ["pytest unit test"],
            "forbidden_changes": ["do not edit auth"],
            "implementation_hints": ["keep changes scoped"],
        },
    )

    assert error is None


def test_context_builder_behavior_smoke_rejects_empty_relevant_files() -> None:
    error = evaluate_behavior_smoke(
        LiveSmokeActor.CONTEXT_BUILDER,
        {
            "relevant_files": [],
            "repo_patterns": ["pattern"],
            "test_patterns": ["test"],
            "forbidden_changes": ["forbidden"],
            "implementation_hints": ["hint"],
        },
    )

    assert error == "context_builder output must include non-empty relevant_files"


@pytest.mark.parametrize(
    "owning_surface",
    [
        OwningSurface.FIXTURE,
        OwningSurface.PROVIDER,
        OwningSurface.OPERATOR_ENVIRONMENT,
        OwningSurface.UNKNOWN,
    ],
)
def test_repair_proposal_rejects_non_repairable_owning_surfaces(
    owning_surface: OwningSurface,
) -> None:
    with pytest.raises(ValidationError):
        RepairProposal(
            owning_surface=owning_surface,
            file_paths=[".harness/actors/context_builder.md"],
            summary="Do not propose non-Rail-owned repairs.",
            preserves_fail_closed_policy=True,
        )


@pytest.mark.parametrize(
    "file_path",
    [
        "/tmp/rail/.harness/actors/context_builder.md",
        "../rail/.harness/actors/context_builder.md",
        ".harness/actors/../supervisor/routing.yaml",
        "target/app/service.py",
        "src/rail/auth/credentials.py",
        ".harness/artifacts/runs/attempt-0001/evidence.json",
        "smoke_reports/planner/report.json",
        "docs/SPEC.md",
    ],
)
def test_repair_proposal_rejects_forbidden_file_paths(file_path: str) -> None:
    with pytest.raises(ValidationError):
        RepairProposal(
            owning_surface=OwningSurface.ACTOR_PROMPT,
            file_paths=[file_path],
            summary="Do not propose unsafe repair targets.",
            preserves_fail_closed_policy=True,
        )


def test_repair_proposal_rejects_empty_file_paths() -> None:
    with pytest.raises(ValidationError):
        RepairProposal(
            owning_surface=OwningSurface.ACTOR_PROMPT,
            file_paths=[],
            summary="Do not propose repairs without concrete files.",
            preserves_fail_closed_policy=True,
        )


def test_repair_proposal_requires_fail_closed_preservation() -> None:
    with pytest.raises(ValidationError):
        RepairProposal(
            owning_surface=OwningSurface.ACTOR_PROMPT,
            file_paths=[".harness/actors/context_builder.md"],
            summary="Do not weaken fail-closed policy.",
            preserves_fail_closed_policy=False,
        )


def test_symptom_classes_include_non_actor_environment_failures() -> None:
    assert SymptomClass.READINESS_FAILURE.value == "readiness_failure"
    assert SymptomClass.PROVIDER_TRANSIENT_FAILURE.value == "provider_transient_failure"
    assert SymptomClass.EVIDENCE_WRITER_FAILURE.value == "evidence_writer_failure"
