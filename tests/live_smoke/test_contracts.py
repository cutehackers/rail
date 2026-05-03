from __future__ import annotations

from pathlib import Path

import pytest
from pydantic import ValidationError

from rail.artifacts.digests import digest_payload
from rail.live_smoke.contracts import LIVE_SMOKE_ACTORS, V1_LIVE_SMOKE_ACTORS, evaluate_behavior_smoke
from rail.live_smoke.models import (
    LiveSmokeActor,
    LiveSmokeReport,
    LiveSmokeVerdict,
    OwningSurface,
    RepairProposal,
    SymptomClass,
)
from rail.live_smoke.seeds import build_live_smoke_seed, canonical_prior_outputs_for


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
        artifact_id="rail-abc",
        artifact_dir=tmp_path / ".harness" / "artifacts" / "rail-abc",
        report_dir=tmp_path,
        fixture_digest="sha256:abc",
        evidence_refs=["runs/attempt-0001/planner.events.jsonl"],
        repair_proposal=None,
    )

    assert report.verdict == LiveSmokeVerdict.PASSED
    assert report.artifact_id == "rail-abc"


def test_live_smoke_report_rejects_evidence_without_artifact_metadata(
    tmp_path: Path,
) -> None:
    with pytest.raises(ValidationError):
        LiveSmokeReport(
            actor=LiveSmokeActor.PLANNER,
            verdict=LiveSmokeVerdict.PASSED,
            symptom_class=None,
            owning_surface=None,
            report_dir=tmp_path,
            fixture_digest="sha256:abc",
            evidence_refs=["runs/attempt-0001/planner.events.jsonl"],
            repair_proposal=None,
        )


def test_live_smoke_report_requires_complete_artifact_metadata(
    tmp_path: Path,
) -> None:
    with pytest.raises(ValidationError):
        LiveSmokeReport(
            actor=LiveSmokeActor.PLANNER,
            verdict=LiveSmokeVerdict.PASSED,
            symptom_class=None,
            owning_surface=None,
            artifact_id="rail-abc",
            report_dir=tmp_path,
            fixture_digest="sha256:abc",
            evidence_refs=[],
            repair_proposal=None,
        )


def test_live_smoke_report_requires_complete_seed_metadata(
    tmp_path: Path,
) -> None:
    with pytest.raises(ValidationError):
        LiveSmokeReport(
            actor=LiveSmokeActor.PLANNER,
            verdict=LiveSmokeVerdict.PASSED,
            symptom_class=None,
            owning_surface=None,
            report_dir=tmp_path,
            fixture_digest="sha256:abc",
            seed_schema_version="1",
            evidence_refs=[],
            repair_proposal=None,
        )


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


def test_v1_live_smoke_actor_scope_remains_initial_pair() -> None:
    assert V1_LIVE_SMOKE_ACTORS == (
        LiveSmokeActor.PLANNER,
        LiveSmokeActor.CONTEXT_BUILDER,
    )


def test_live_smoke_actor_scope_includes_all_supervisor_actors() -> None:
    assert LIVE_SMOKE_ACTORS == (
        LiveSmokeActor.PLANNER,
        LiveSmokeActor.CONTEXT_BUILDER,
        LiveSmokeActor.CRITIC,
        LiveSmokeActor.GENERATOR,
        LiveSmokeActor.EXECUTOR,
        LiveSmokeActor.EVALUATOR,
    )


def test_planner_behavior_smoke_requires_minimum_fields() -> None:
    error = evaluate_behavior_smoke(
        LiveSmokeActor.PLANNER,
        {
            "summary": "Plan",
            "substeps": [],
            "risks": [],
            "acceptance_criteria_refined": [],
        },
    )

    assert error is None


@pytest.mark.parametrize(
    "missing_field",
    [
        "summary",
        "substeps",
        "risks",
        "acceptance_criteria_refined",
    ],
)
def test_planner_behavior_smoke_reports_missing_required_field(
    missing_field: str,
) -> None:
    output = {
        "summary": "Plan",
        "substeps": [],
        "risks": [],
        "acceptance_criteria_refined": [],
    }
    del output[missing_field]

    error = evaluate_behavior_smoke(LiveSmokeActor.PLANNER, output)

    assert error == f"planner output must include {missing_field}"


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


@pytest.mark.parametrize(
    "missing_field",
    [
        "relevant_files",
        "repo_patterns",
        "test_patterns",
        "forbidden_changes",
        "implementation_hints",
    ],
)
def test_context_builder_behavior_smoke_reports_missing_required_field(
    missing_field: str,
) -> None:
    output = {
        "relevant_files": [{"path": "README.md", "why": "entry point"}],
        "repo_patterns": ["small service module"],
        "test_patterns": ["pytest unit test"],
        "forbidden_changes": ["do not edit auth"],
        "implementation_hints": ["keep changes scoped"],
    }
    del output[missing_field]

    error = evaluate_behavior_smoke(LiveSmokeActor.CONTEXT_BUILDER, output)

    assert error == f"context_builder output must include {missing_field}"


@pytest.mark.parametrize(
    "empty_field",
    [
        "relevant_files",
        "repo_patterns",
        "forbidden_changes",
        "implementation_hints",
    ],
)
def test_context_builder_behavior_smoke_rejects_empty_required_context(
    empty_field: str,
) -> None:
    output = {
        "relevant_files": [{"path": "README.md", "why": "entry point"}],
        "repo_patterns": ["small service module"],
        "test_patterns": ["pytest unit test"],
        "forbidden_changes": ["do not edit auth"],
        "implementation_hints": ["keep changes scoped"],
    }
    output[empty_field] = []

    error = evaluate_behavior_smoke(
        LiveSmokeActor.CONTEXT_BUILDER,
        output,
    )

    assert error == f"context_builder output must include non-empty {empty_field}"


def test_critic_behavior_smoke_requires_non_empty_guardrails() -> None:
    error = evaluate_behavior_smoke(
        LiveSmokeActor.CRITIC,
        {
            "priority_focus": ["Patch bundle boundary"],
            "missing_requirements": [],
            "risk_hypotheses": [],
            "validation_expectations": ["Focused fixture test"],
            "generator_guardrails": ["Do not mutate target files directly"],
            "blocked_assumptions": [],
        },
    )

    assert error is None


def test_critic_behavior_smoke_rejects_empty_guardrails() -> None:
    error = evaluate_behavior_smoke(
        LiveSmokeActor.CRITIC,
        {
            "priority_focus": ["Patch bundle boundary"],
            "missing_requirements": [],
            "risk_hypotheses": [],
            "validation_expectations": ["Focused fixture test"],
            "generator_guardrails": [],
            "blocked_assumptions": [],
        },
    )

    assert error == "critic output must include non-empty generator_guardrails"


def test_generator_behavior_smoke_validates_inline_patch_bundle(tmp_path: Path) -> None:
    target = tmp_path / "target"
    target.mkdir()
    (target / "app").mkdir()
    (target / "app" / "service.py").write_text("def normalize_title(value):\n    return value\n", encoding="utf-8")
    fixture_digest = digest_payload({"fixture": "stable"})
    seed = build_live_smoke_seed(
        LiveSmokeActor.GENERATOR,
        fixture_digest=fixture_digest,
        prior_outputs=canonical_prior_outputs_for(LiveSmokeActor.GENERATOR),
    )
    output = {
        "changed_files": ["app/service.py"],
        "patch_summary": ["Updated service behavior"],
        "tests_added_or_updated": [],
        "known_limits": [],
        "patch_bundle": {
            "schema_version": "1",
            "base_tree_digest": fixture_digest,
            "operations": [
                {"op": "write", "path": "app/service.py", "content": "def normalize_title(value):\n    return value\n"},
            ],
        },
    }

    error = evaluate_behavior_smoke(
        LiveSmokeActor.GENERATOR,
        output,
        seed=seed,
    )

    assert error is None


def test_generator_behavior_smoke_rejects_missing_seeded_patch_path() -> None:
    fixture_digest = "sha256:fixture"
    seed = build_live_smoke_seed(
        LiveSmokeActor.GENERATOR,
        fixture_digest=fixture_digest,
        prior_outputs=canonical_prior_outputs_for(LiveSmokeActor.GENERATOR),
    )

    error = evaluate_behavior_smoke(
        LiveSmokeActor.GENERATOR,
        {
            "changed_files": [],
            "patch_summary": [],
            "tests_added_or_updated": [],
            "known_limits": [],
        },
        seed=seed,
    )

    assert error == "generator output must include patch_bundle for seeded patch smoke"


def test_generator_behavior_smoke_rejects_wrong_patch_base_digest() -> None:
    seed = build_live_smoke_seed(
        LiveSmokeActor.GENERATOR,
        fixture_digest="sha256:fixture",
        prior_outputs=canonical_prior_outputs_for(LiveSmokeActor.GENERATOR),
    )

    error = evaluate_behavior_smoke(
        LiveSmokeActor.GENERATOR,
        {
            "changed_files": ["app/service.py"],
            "patch_summary": ["Updated service behavior"],
            "tests_added_or_updated": [],
            "known_limits": [],
            "patch_bundle": {
                "schema_version": "1",
                "base_tree_digest": "sha256:other",
                "operations": [
                    {"op": "write", "path": "app/service.py", "content": "content"},
                ],
            },
        },
        seed=seed,
    )

    assert error == "generator patch_bundle base_tree_digest must match live smoke fixture_digest"


def test_executor_behavior_smoke_requires_consistent_test_counts() -> None:
    error = evaluate_behavior_smoke(
        LiveSmokeActor.EXECUTOR,
        {
            "format": "pass",
            "analyze": "pass",
            "tests": {"total": 2, "passed": 1, "failed": 0},
            "failure_details": [],
            "logs": ["ran focused validation"],
        },
    )

    assert error == "executor output test counts must be internally consistent"


def test_executor_behavior_smoke_accepts_machine_readable_failure_class() -> None:
    error = evaluate_behavior_smoke(
        LiveSmokeActor.EXECUTOR,
        {
            "format": "fail",
            "analyze": "pass",
            "tests": {"total": 1, "passed": 0, "failed": 1},
            "failure_details": ["class=validation_failure pytest failed"],
            "logs": ["pytest failed"],
        },
    )

    assert error is None


def test_evaluator_behavior_smoke_requires_digest_echo() -> None:
    error = evaluate_behavior_smoke(
        LiveSmokeActor.EVALUATOR,
        {
            "decision": "pass",
            "evaluated_input_digest": "sha256:wrong",
            "findings": [],
            "reason_codes": [],
            "quality_confidence": "high",
        },
        invocation_input={"evaluator_input_digest": "sha256:expected"},
    )

    assert error == "evaluator output must echo evaluator_input_digest"


def test_evaluator_behavior_smoke_accepts_revise_with_next_action() -> None:
    error = evaluate_behavior_smoke(
        LiveSmokeActor.EVALUATOR,
        {
            "decision": "revise",
            "evaluated_input_digest": "sha256:expected",
            "findings": ["Validation failed"],
            "reason_codes": ["validation_requirement_fixture"],
            "quality_confidence": "medium",
            "next_action": "revise_generator",
        },
        invocation_input={"evaluator_input_digest": "sha256:expected"},
    )

    assert error is None


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
