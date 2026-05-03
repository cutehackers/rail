from __future__ import annotations

from enum import StrEnum
from pathlib import Path
from typing import Literal, Self

from pydantic import BaseModel, ConfigDict, model_validator

from rail.live_smoke.models import LiveSmokeActor, OwningSurface, RepairProposal, SymptomClass
from rail.workspace.patch_bundle import PatchBundle, validate_patch_bundle


class RepairRiskLevel(StrEnum):
    LOW = "low"
    MEDIUM = "medium"
    HIGH = "high"


class RepairLoopStatus(StrEnum):
    PASSED = "passed"
    CANDIDATE_READY = "candidate_ready"
    REPAIRED = "repaired"
    UNREPAIRABLE = "unrepairable"
    BUDGET_EXHAUSTED = "budget_exhausted"
    FAILED_VALIDATION = "failed_validation"


class RepairCandidate(BaseModel):
    model_config = ConfigDict(extra="forbid")

    schema_version: Literal["1"] = "1"
    actor: LiveSmokeActor
    symptom_class: SymptomClass
    owning_surface: OwningSurface
    source_report_path: Path
    evidence_refs: list[str]
    file_paths: list[str]
    summary: str
    risk_level: RepairRiskLevel
    patch_bundle: PatchBundle
    validation_commands: list[str]
    preserves_fail_closed_policy: bool
    auto_apply: bool = False

    @model_validator(mode="after")
    def validate_repair_candidate(self) -> Self:
        RepairProposal(
            owning_surface=self.owning_surface,
            file_paths=self.file_paths,
            summary=self.summary,
            preserves_fail_closed_policy=self.preserves_fail_closed_policy,
        )
        if not self.preserves_fail_closed_policy:
            raise ValueError("repair candidates must preserve fail-closed policy")
        if self.risk_level == RepairRiskLevel.HIGH and self.auto_apply:
            raise ValueError("high-risk repair candidates must not be auto-applied")
        try:
            validate_patch_bundle(self.patch_bundle)
        except ValueError as exc:
            raise ValueError(f"patch bundle is invalid: {exc}") from exc

        declared_paths = set(self.file_paths)
        for operation in self.patch_bundle.operations:
            if operation.path not in declared_paths:
                raise ValueError("patch operations must target declared file_paths")
        return self


class RepairIterationReport(BaseModel):
    model_config = ConfigDict(extra="forbid")

    iteration: int
    actor: LiveSmokeActor
    report_path: Path
    candidate: RepairCandidate | None
    applied_patch_digest: str | None
    pre_apply_tree_digest: str | None
    post_apply_tree_digest: str | None

    @model_validator(mode="after")
    def validate_applied_digest_group(self) -> Self:
        applied_fields = (
            self.applied_patch_digest,
            self.pre_apply_tree_digest,
            self.post_apply_tree_digest,
        )
        if any(value is not None for value in applied_fields) and not all(value is not None for value in applied_fields):
            raise ValueError("applied patch digest and tree digests must be recorded together")
        return self


class LiveSmokeRepairLoopReport(BaseModel):
    model_config = ConfigDict(extra="forbid")

    schema_version: Literal["1"] = "1"
    status: RepairLoopStatus
    actors: list[LiveSmokeActor]
    report_dir: Path
    apply: bool
    max_iterations: int
    iterations: list[RepairIterationReport]

    @model_validator(mode="after")
    def validate_loop_report_state(self) -> Self:
        if not self.actors:
            raise ValueError("repair loop report requires actors")
        if self.max_iterations < 1:
            raise ValueError("max_iterations must be at least 1")
        if self.status in {RepairLoopStatus.PASSED, RepairLoopStatus.REPAIRED}:
            for iteration in self.iterations:
                if iteration.candidate is not None and iteration.applied_patch_digest is None:
                    raise ValueError("passed or repaired reports must not carry unapplied candidates")
        return self
