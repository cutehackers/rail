from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, ConfigDict


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
    findings: list[str]
    reason_codes: list[str]
    quality_confidence: Literal["high", "medium", "low"]
    next_action: str | None = None


ACTOR_OUTPUT_MODELS = {
    "planner": PlanOutput,
    "context_builder": ContextPackOutput,
    "critic": CriticReportOutput,
    "generator": ImplementationResultOutput,
    "executor": ExecutionReportOutput,
    "evaluator": EvaluationResultOutput,
}


def fake_actor_output(actor: str) -> dict[str, object]:
    outputs: dict[str, dict[str, object]] = {
        "planner": {
            "summary": "Plan one bounded step.",
            "likely_files": ["src/rail/api.py"],
            "substeps": ["Inspect", "Implement", "Verify"],
            "risks": ["Incomplete validation"],
            "acceptance_criteria_refined": ["Tests pass"],
        },
        "context_builder": {
            "relevant_files": [{"path": "src/rail/api.py", "why": "Public API"}],
            "repo_patterns": ["Python package under src"],
            "forbidden_changes": ["Do not mutate target directly"],
        },
        "critic": {
            "priority_focus": ["Policy boundary"],
            "missing_requirements": [],
            "risk_hypotheses": [],
            "validation_expectations": ["pytest"],
            "generator_guardrails": ["patch only"],
            "blocked_assumptions": [],
        },
        "generator": {
            "changed_files": ["src/rail/api.py"],
            "patch_summary": ["Added API"],
            "tests_added_or_updated": ["tests/test_api_smoke.py"],
            "known_limits": [],
            "patch_bundle_ref": "patches/generator.patch.yaml",
        },
        "executor": {
            "format": "pass",
            "analyze": "pass",
            "tests": {"total": 1, "passed": 1, "failed": 0},
            "failure_details": [],
            "logs": ["pytest passed"],
        },
        "evaluator": {
            "decision": "pass",
            "findings": [],
            "reason_codes": [],
            "quality_confidence": "high",
        },
    }
    return outputs[actor]
