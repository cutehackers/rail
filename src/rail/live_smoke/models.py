from __future__ import annotations

from enum import StrEnum
from pathlib import Path, PurePosixPath
from typing import Self

from pydantic import BaseModel, ConfigDict, field_validator, model_validator


class LiveSmokeActor(StrEnum):
    PLANNER = "planner"
    CONTEXT_BUILDER = "context_builder"


class LiveSmokeVerdict(StrEnum):
    PASSED = "passed"
    FAILED = "failed"


class SymptomClass(StrEnum):
    READINESS_FAILURE = "readiness_failure"
    PROVIDER_TRANSIENT_FAILURE = "provider_transient_failure"
    POLICY_VIOLATION = "policy_violation"
    SCHEMA_MISMATCH = "schema_mismatch"
    FIXTURE_DIGEST_MISMATCH = "fixture_digest_mismatch"
    FIXTURE_PREP_FAILURE = "fixture_prep_failure"
    EVIDENCE_WRITER_FAILURE = "evidence_writer_failure"
    BEHAVIOR_SMOKE_FAILURE = "behavior_smoke_failure"
    UNKNOWN_FAILURE = "unknown_failure"


class OwningSurface(StrEnum):
    ACTOR_PROMPT = "actor_prompt"
    RUNTIME_INVOCATION = "runtime_invocation"
    RUNTIME_CONTRACT = "runtime_contract"
    PACKAGED_ASSET = "packaged_asset"
    FIXTURE = "fixture"
    PROVIDER = "provider"
    OPERATOR_ENVIRONMENT = "operator_environment"
    UNKNOWN = "unknown"


_REPAIRABLE_OWNING_SURFACES = frozenset(
    {
        OwningSurface.ACTOR_PROMPT,
        OwningSurface.RUNTIME_INVOCATION,
        OwningSurface.RUNTIME_CONTRACT,
        OwningSurface.PACKAGED_ASSET,
    }
)
_ALLOWED_REPAIR_PATH_DIRS = (
    ".harness/actors",
    ".harness/templates",
    "src/rail/actor_runtime",
    "src/rail/live_smoke",
    "src/rail/package_assets",
)
_ALLOWED_REPAIR_PATHS = frozenset(
    {
        "pyproject.toml",
        "scripts/check_python_package_assets.py",
    }
)
_FORBIDDEN_REPAIR_PATH_PARTS = frozenset(
    {
        ".git",
        ".worktrees",
        "artifacts",
        "auth",
        "evidence",
        "fixture_target",
        "live_smoke_reports",
        "smoke_reports",
        "target",
        "target_repo",
    }
)


def _is_allowed_repair_path(file_path: str) -> bool:
    if file_path in _ALLOWED_REPAIR_PATHS:
        return True
    return any(
        file_path == allowed_dir or file_path.startswith(f"{allowed_dir}/")
        for allowed_dir in _ALLOWED_REPAIR_PATH_DIRS
    )


class RepairProposal(BaseModel):
    model_config = ConfigDict(extra="forbid")

    owning_surface: OwningSurface
    file_paths: list[str]
    summary: str
    preserves_fail_closed_policy: bool

    @field_validator("file_paths")
    @classmethod
    def validate_file_paths(cls, file_paths: list[str]) -> list[str]:
        for file_path in file_paths:
            if "\\" in file_path:
                raise ValueError("repair file paths must use POSIX separators")
            if not file_path:
                raise ValueError("repair file paths must not be empty")

            path = PurePosixPath(file_path)
            raw_parts = file_path.split("/")
            if path.is_absolute():
                raise ValueError("repair file paths must be relative")
            if ".." in path.parts or "." in raw_parts or "" in raw_parts:
                raise ValueError("repair file paths must not contain traversal")
            if any(part in _FORBIDDEN_REPAIR_PATH_PARTS for part in path.parts):
                raise ValueError("repair file paths must not target forbidden surfaces")
            if not _is_allowed_repair_path(path.as_posix()):
                raise ValueError("repair file paths must target Rail-owned repair surfaces")

        return file_paths

    @model_validator(mode="after")
    def validate_safe_repair(self) -> Self:
        if self.owning_surface not in _REPAIRABLE_OWNING_SURFACES:
            raise ValueError("repair proposal owning surface is not repairable")
        if not self.preserves_fail_closed_policy:
            raise ValueError("repair proposals must preserve fail-closed policy")
        return self


class LiveSmokeReport(BaseModel):
    model_config = ConfigDict(extra="forbid")

    actor: LiveSmokeActor
    verdict: LiveSmokeVerdict
    symptom_class: SymptomClass | None
    owning_surface: OwningSurface | None
    report_dir: Path
    fixture_digest: str
    evidence_refs: list[str]
    repair_proposal: RepairProposal | None

    @model_validator(mode="after")
    def validate_verdict_state(self) -> Self:
        if self.verdict == LiveSmokeVerdict.PASSED:
            if self.symptom_class is not None:
                raise ValueError("passed reports must not carry symptom_class")
            if self.owning_surface is not None:
                raise ValueError("passed reports must not carry owning_surface")
            if self.repair_proposal is not None:
                raise ValueError("passed reports must not carry repair_proposal")

        if self.verdict == LiveSmokeVerdict.FAILED:
            if self.symptom_class is None:
                raise ValueError("failed reports must carry symptom_class")
            if self.owning_surface is None:
                raise ValueError("failed reports must carry owning_surface")

        return self
