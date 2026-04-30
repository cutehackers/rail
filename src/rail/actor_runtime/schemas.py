from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, ConfigDict, field_validator, model_validator

from rail.workspace.patch_bundle import PatchBundle


class PlanOutput(BaseModel):
    model_config = ConfigDict(extra="forbid")

    summary: str
    likely_files: list[str]
    assumptions: list[str] = []
    substeps: list[str]
    risks: list[str]
    acceptance_criteria_refined: list[str]


class RelevantFile(BaseModel):
    model_config = ConfigDict(extra="forbid")

    path: str
    why: str


class ContextPackOutput(BaseModel):
    model_config = ConfigDict(extra="forbid")

    relevant_files: list[RelevantFile]
    repo_patterns: list[str]
    test_patterns: list[str] = []
    forbidden_changes: list[str]
    implementation_hints: list[str] = []


class CriticReportOutput(BaseModel):
    model_config = ConfigDict(extra="forbid")

    priority_focus: list[str]
    missing_requirements: list[str]
    risk_hypotheses: list[str]
    validation_expectations: list[str]
    generator_guardrails: list[str]
    blocked_assumptions: list[str]


class ImplementationResultOutput(BaseModel):
    model_config = ConfigDict(extra="forbid")

    changed_files: list[str]
    patch_summary: list[str]
    tests_added_or_updated: list[str]
    known_limits: list[str]
    patch_bundle_ref: str | None = None
    patch_bundle: PatchBundle | None = None

    @model_validator(mode="after")
    def _single_patch_source(self) -> ImplementationResultOutput:
        if self.patch_bundle_ref and self.patch_bundle is not None:
            raise ValueError("generator output must include exactly one patch source or no patch when read-only")
        return self


class ExecutionTestsOutput(BaseModel):
    model_config = ConfigDict(extra="forbid")

    total: int
    passed: int
    failed: int


class ExecutionReportOutput(BaseModel):
    model_config = ConfigDict(extra="allow")

    format: Literal["pass", "fail"]
    analyze: Literal["pass", "fail"]
    tests: ExecutionTestsOutput
    failure_details: list[str]
    logs: list[str]


class EvaluationResultOutput(BaseModel):
    model_config = ConfigDict(extra="forbid")

    decision: Literal["pass", "revise", "reject"]
    evaluated_input_digest: str
    findings: list[str]
    reason_codes: list[str]
    quality_confidence: Literal["high", "medium", "low"]
    next_action: str | None = None

    @field_validator("evaluated_input_digest")
    @classmethod
    def _sha256_digest(cls, value: str) -> str:
        if not value.startswith("sha256:"):
            raise ValueError("evaluated_input_digest must start with sha256:")
        return value


ACTOR_OUTPUT_MODELS: dict[str, type[BaseModel]] = {
    "planner": PlanOutput,
    "context_builder": ContextPackOutput,
    "critic": CriticReportOutput,
    "generator": ImplementationResultOutput,
    "executor": ExecutionReportOutput,
    "evaluator": EvaluationResultOutput,
}
