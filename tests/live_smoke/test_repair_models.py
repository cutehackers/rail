from __future__ import annotations

from pathlib import Path

import pytest
from pydantic import ValidationError

from rail.live_smoke.models import LiveSmokeActor, OwningSurface, SymptomClass
from rail.live_smoke.repair_models import (
    LiveSmokeRepairLoopReport,
    RepairCandidate,
    RepairIterationReport,
    RepairLoopStatus,
    RepairRiskLevel,
)
from rail.workspace.patch_bundle import PatchBundle


def _patch_bundle() -> PatchBundle:
    return PatchBundle(
        base_tree_digest="sha256:base",
        operations=[
            {
                "op": "write",
                "path": ".harness/actors/generator.md",
                "content": "You are the Generator actor.\n",
                "binary": False,
                "executable": False,
            }
        ],
    )


def test_repair_candidate_accepts_safe_rail_owned_patch(tmp_path: Path) -> None:
    candidate = RepairCandidate(
        actor=LiveSmokeActor.GENERATOR,
        symptom_class=SymptomClass.POLICY_VIOLATION,
        owning_surface=OwningSurface.RUNTIME_CONTRACT,
        source_report_path=tmp_path / "live_smoke_report.json",
        evidence_refs=["runs/attempt-0001/generator.runtime_evidence.json"],
        file_paths=[".harness/actors/generator.md"],
        summary="Guide generator away from forbidden tool probes.",
        risk_level=RepairRiskLevel.LOW,
        patch_bundle=_patch_bundle(),
        validation_commands=["uv run --python 3.12 pytest tests/live_smoke/test_runner.py -q"],
        preserves_fail_closed_policy=True,
        auto_apply=True,
    )

    assert candidate.schema_version == "1"
    assert candidate.patch_bundle.operations[0].path == ".harness/actors/generator.md"


@pytest.mark.parametrize(
    "file_path",
    [
        "../src/rail/live_smoke/runner.py",
        ".harness/artifacts/run.txt",
        "target/app.py",
        "/absolute/path.py",
    ],
)
def test_repair_candidate_rejects_unsafe_file_paths(tmp_path: Path, file_path: str) -> None:
    with pytest.raises(ValidationError):
        RepairCandidate(
            actor=LiveSmokeActor.GENERATOR,
            symptom_class=SymptomClass.POLICY_VIOLATION,
            owning_surface=OwningSurface.RUNTIME_CONTRACT,
            source_report_path=tmp_path / "live_smoke_report.json",
            evidence_refs=[],
            file_paths=[file_path],
            summary="Unsafe repair path.",
            risk_level=RepairRiskLevel.LOW,
            patch_bundle=_patch_bundle(),
            validation_commands=[],
            preserves_fail_closed_policy=True,
        )


def test_repair_candidate_rejects_patch_path_outside_declared_files(tmp_path: Path) -> None:
    with pytest.raises(ValidationError, match="patch operations must target declared file_paths"):
        RepairCandidate(
            actor=LiveSmokeActor.GENERATOR,
            symptom_class=SymptomClass.POLICY_VIOLATION,
            owning_surface=OwningSurface.RUNTIME_CONTRACT,
            source_report_path=tmp_path / "live_smoke_report.json",
            evidence_refs=[],
            file_paths=[".harness/actors/executor.md"],
            summary="Mismatch.",
            risk_level=RepairRiskLevel.LOW,
            patch_bundle=_patch_bundle(),
            validation_commands=[],
            preserves_fail_closed_policy=True,
        )


def test_repair_candidate_rejects_high_risk_auto_apply(tmp_path: Path) -> None:
    with pytest.raises(ValidationError, match="high-risk"):
        RepairCandidate(
            actor=LiveSmokeActor.GENERATOR,
            symptom_class=SymptomClass.POLICY_VIOLATION,
            owning_surface=OwningSurface.RUNTIME_CONTRACT,
            source_report_path=tmp_path / "live_smoke_report.json",
            evidence_refs=[],
            file_paths=[".harness/actors/generator.md"],
            summary="Risky.",
            risk_level=RepairRiskLevel.HIGH,
            patch_bundle=_patch_bundle(),
            validation_commands=[],
            preserves_fail_closed_policy=True,
            auto_apply=True,
        )


def test_repair_candidate_requires_fail_closed_policy(tmp_path: Path) -> None:
    with pytest.raises(ValidationError, match="fail-closed"):
        RepairCandidate(
            actor=LiveSmokeActor.GENERATOR,
            symptom_class=SymptomClass.POLICY_VIOLATION,
            owning_surface=OwningSurface.RUNTIME_CONTRACT,
            source_report_path=tmp_path / "live_smoke_report.json",
            evidence_refs=[],
            file_paths=[".harness/actors/generator.md"],
            summary="Unsafe.",
            risk_level=RepairRiskLevel.LOW,
            patch_bundle=_patch_bundle(),
            validation_commands=[],
            preserves_fail_closed_policy=False,
        )


def test_repair_candidate_rejects_invalid_patch_bundle(tmp_path: Path) -> None:
    bundle = PatchBundle(
        base_tree_digest="sha256:base",
        operations=[
            {
                "op": "write",
                "path": ".harness/artifacts/bad.txt",
                "content": "bad\n",
                "binary": False,
                "executable": False,
            }
        ],
    )

    with pytest.raises(ValidationError, match="patch bundle"):
        RepairCandidate(
            actor=LiveSmokeActor.GENERATOR,
            symptom_class=SymptomClass.POLICY_VIOLATION,
            owning_surface=OwningSurface.RUNTIME_CONTRACT,
            source_report_path=tmp_path / "live_smoke_report.json",
            evidence_refs=[],
            file_paths=[".harness/actors/generator.md"],
            summary="Invalid patch.",
            risk_level=RepairRiskLevel.LOW,
            patch_bundle=bundle,
            validation_commands=[],
            preserves_fail_closed_policy=True,
        )


def test_repair_loop_report_rejects_inconsistent_passed_state(tmp_path: Path) -> None:
    with pytest.raises(ValidationError, match="passed"):
        LiveSmokeRepairLoopReport(
            status=RepairLoopStatus.PASSED,
            actors=[LiveSmokeActor.GENERATOR],
            report_dir=tmp_path,
            apply=True,
            max_iterations=2,
            iterations=[
                RepairIterationReport(
                    iteration=1,
                    actor=LiveSmokeActor.GENERATOR,
                    report_path=tmp_path / "live_smoke_report.json",
                    candidate=RepairCandidate(
                        actor=LiveSmokeActor.GENERATOR,
                        symptom_class=SymptomClass.POLICY_VIOLATION,
                        owning_surface=OwningSurface.RUNTIME_CONTRACT,
                        source_report_path=tmp_path / "live_smoke_report.json",
                        evidence_refs=[],
                        file_paths=[".harness/actors/generator.md"],
                        summary="Unapplied repair.",
                        risk_level=RepairRiskLevel.LOW,
                        patch_bundle=_patch_bundle(),
                        validation_commands=[],
                        preserves_fail_closed_policy=True,
                    ),
                    applied_patch_digest=None,
                    pre_apply_tree_digest=None,
                    post_apply_tree_digest=None,
                )
            ],
        )
