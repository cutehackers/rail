from __future__ import annotations

from pathlib import Path

from rail.live_smoke.models import LiveSmokeActor, OwningSurface, SymptomClass
from rail.live_smoke.repair_evidence import RepairEvidenceSummary
from rail.live_smoke.repair_models import RepairRiskLevel
from rail.live_smoke.repairers import build_repair_candidate, repairer_registry


def _summary(
    *,
    actor: LiveSmokeActor = LiveSmokeActor.GENERATOR,
    symptom_class: SymptomClass = SymptomClass.POLICY_VIOLATION,
    owning_surface: OwningSurface = OwningSurface.RUNTIME_CONTRACT,
    error_text: str = "shell executable is not allowed: python",
    policy_violation_reason: str | None = "shell executable is not allowed: python",
) -> RepairEvidenceSummary:
    return RepairEvidenceSummary(
        actor=actor,
        report_path=Path("reports/generator/live_smoke_report.json"),
        symptom_class=symptom_class,
        owning_surface=owning_surface,
        error_text=error_text,
        policy_violation_reason=policy_violation_reason,
        output_schema_ref="actor_runtime/schemas/generator.schema.json",
        output_schema_digest="sha256:schema",
        evidence_refs=["runs/attempt-0001/generator.runtime_evidence.json"],
        seed_digest="sha256:seed",
        fixture_digest="sha256:fixture",
    )


def test_repairer_registry_includes_known_repairable_failure_classes() -> None:
    registry = repairer_registry()

    assert (SymptomClass.POLICY_VIOLATION, OwningSurface.RUNTIME_CONTRACT) in registry
    assert (SymptomClass.SCHEMA_MISMATCH, OwningSurface.ACTOR_PROMPT) in registry
    assert (SymptomClass.BEHAVIOR_SMOKE_FAILURE, OwningSurface.ACTOR_PROMPT) in registry


def test_shell_policy_repairer_adds_prompt_guidance_without_widening_policy(tmp_path: Path) -> None:
    prompt = tmp_path / ".harness" / "actors" / "generator.md"
    prompt.parent.mkdir(parents=True)
    prompt.write_text("You are the Generator actor.\n", encoding="utf-8")

    candidate = build_repair_candidate(_summary(), repo_root=tmp_path)

    assert candidate is not None
    assert candidate.risk_level == RepairRiskLevel.LOW
    assert candidate.file_paths == [".harness/actors/generator.md"]
    assert "allowlist" not in candidate.patch_bundle.operations[0].content.lower()
    assert "do not probe unavailable tools" in candidate.patch_bundle.operations[0].content


def test_schema_drift_repairer_updates_all_schema_template_copies(tmp_path: Path) -> None:
    schema_text = """type: object
properties:
  patch_bundle:
    type: object
    properties:
      operations:
        type: array
        items:
          type: object
          required:
            - op
            - path
            - content
          properties:
            executable:
              type: boolean
            binary:
              type: boolean
"""
    for path in [
        ".harness/templates/implementation_result.schema.yaml",
        "assets/defaults/templates/implementation_result.schema.yaml",
        "src/rail/package_assets/defaults/templates/implementation_result.schema.yaml",
    ]:
        target = tmp_path / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(schema_text, encoding="utf-8")

    candidate = build_repair_candidate(
        _summary(
            symptom_class=SymptomClass.SCHEMA_MISMATCH,
            owning_surface=OwningSurface.ACTOR_PROMPT,
            error_text="patch_bundle.operations.0.executable Input should be a valid boolean",
            policy_violation_reason=None,
        ),
        repo_root=tmp_path,
    )

    assert candidate is not None
    assert candidate.risk_level == RepairRiskLevel.MEDIUM
    assert set(candidate.file_paths) == {
        ".harness/templates/implementation_result.schema.yaml",
        "assets/defaults/templates/implementation_result.schema.yaml",
        "src/rail/package_assets/defaults/templates/implementation_result.schema.yaml",
    }
    for operation in candidate.patch_bundle.operations:
        assert "            - executable" in operation.content
        assert "            - binary" in operation.content


def test_behavior_contract_repairer_adds_evaluator_digest_prompt_guidance(tmp_path: Path) -> None:
    prompt = tmp_path / ".harness" / "actors" / "evaluator.md"
    prompt.parent.mkdir(parents=True)
    prompt.write_text("You are the Evaluator actor.\n", encoding="utf-8")

    candidate = build_repair_candidate(
        _summary(
            actor=LiveSmokeActor.EVALUATOR,
            symptom_class=SymptomClass.BEHAVIOR_SMOKE_FAILURE,
            owning_surface=OwningSurface.ACTOR_PROMPT,
            error_text="evaluator output must echo evaluator_input_digest",
            policy_violation_reason=None,
        ),
        repo_root=tmp_path,
    )

    assert candidate is not None
    assert candidate.file_paths == [".harness/actors/evaluator.md"]
    assert "evaluator_input_digest" in candidate.patch_bundle.operations[0].content


def test_repairers_return_none_for_unknown_or_nonrepairable_failures(tmp_path: Path) -> None:
    candidate = build_repair_candidate(
        _summary(
            symptom_class=SymptomClass.PROVIDER_TRANSIENT_FAILURE,
            owning_surface=OwningSurface.PROVIDER,
            error_text="provider timeout",
            policy_violation_reason=None,
        ),
        repo_root=tmp_path,
    )

    assert candidate is None
