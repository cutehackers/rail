from __future__ import annotations

from pathlib import Path
from typing import Literal

from pydantic import BaseModel, ConfigDict

from rail.workspace.validation import load_validation_evidence


class EvaluatorGateInput(BaseModel):
    model_config = ConfigDict(extra="forbid")

    artifact_dir: Path
    request_digest: str
    effective_policy_digest: str
    actor_invocation_digest: str
    patch_bundle_digest: str
    tree_digest: str
    validation_ref: Path | None
    evaluator_input_digest: str


class EvaluatorGateResult(BaseModel):
    model_config = ConfigDict(extra="forbid")

    outcome: Literal["pass", "revise", "reject", "blocked"]
    reason: str
    next_actor: str | None = None


def evaluate_gate(evaluator_output: dict[str, object], gate_input: EvaluatorGateInput) -> EvaluatorGateResult:
    decision = evaluator_output.get("decision")
    if decision == "revise":
        return EvaluatorGateResult(outcome="revise", reason="evaluator requested revision", next_actor="generator")
    if decision == "reject":
        return EvaluatorGateResult(outcome="reject", reason="evaluator rejected the result")
    if decision != "pass":
        return EvaluatorGateResult(outcome="blocked", reason="unknown evaluator decision")

    if gate_input.validation_ref is None:
        return EvaluatorGateResult(outcome="blocked", reason="terminal pass requires validation evidence")

    evidence = load_validation_evidence(gate_input.artifact_dir, gate_input.validation_ref)
    if evidence.status != "pass":
        return EvaluatorGateResult(outcome="blocked", reason="required validation failed")
    if evidence.source not in {"request", "policy"}:
        return EvaluatorGateResult(outcome="blocked", reason="validation evidence source must be request or policy")
    if evidence.patch_digest != gate_input.patch_bundle_digest or evidence.tree_digest != gate_input.tree_digest:
        return EvaluatorGateResult(outcome="blocked", reason="validation evidence is not current for evaluated patch")
    if evidence.mutation_status != "clean":
        return EvaluatorGateResult(outcome="blocked", reason="validation mutated the target")

    return EvaluatorGateResult(outcome="pass", reason="validation evidence accepted")
