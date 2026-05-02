from __future__ import annotations

from enum import StrEnum
from pathlib import Path

from pydantic import BaseModel, ConfigDict


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


class RepairProposal(BaseModel):
    model_config = ConfigDict(extra="forbid")

    owning_surface: OwningSurface
    file_paths: list[str]
    summary: str
    preserves_fail_closed_policy: bool


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
