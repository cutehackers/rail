from __future__ import annotations

import json
from pathlib import Path
from typing import Literal

from pydantic import BaseModel, ConfigDict, Field

from rail.artifacts.digests import digest_payload
from rail.policy.schema import NetworkMode
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
    validation_evidence_digest: str
    evaluator_input_digest: str
    runtime_evidence_refs: list[Path] = Field(default_factory=list)
    expected_validation_network_mode: NetworkMode = "disabled"


class EvaluatorGateResult(BaseModel):
    model_config = ConfigDict(extra="forbid")

    outcome: Literal["pass", "revise", "reject", "blocked"]
    reason: str
    next_actor: str | None = None


def evaluate_gate(evaluator_output: dict[str, object], gate_input: EvaluatorGateInput) -> EvaluatorGateResult:
    decision = evaluator_output.get("decision")
    if decision not in {"pass", "revise", "reject"}:
        return EvaluatorGateResult(outcome="blocked", reason="unknown evaluator decision")

    evaluated_input_digest = evaluator_output.get("evaluated_input_digest")
    if not isinstance(evaluated_input_digest, str) or evaluated_input_digest != gate_input.evaluator_input_digest:
        return EvaluatorGateResult(outcome="blocked", reason="evaluator input digest does not match")

    if decision == "revise":
        return EvaluatorGateResult(outcome="revise", reason="evaluator requested revision", next_actor="generator")
    if decision == "reject":
        return EvaluatorGateResult(outcome="reject", reason="evaluator rejected the result")

    if gate_input.validation_ref is None:
        return EvaluatorGateResult(outcome="blocked", reason="terminal pass requires validation evidence")

    runtime_policy_violation = _runtime_policy_violation_reason(gate_input)
    if runtime_policy_violation is not None:
        return EvaluatorGateResult(outcome="blocked", reason=runtime_policy_violation)

    evidence = load_validation_evidence(gate_input.artifact_dir, gate_input.validation_ref)
    if evidence.status != "pass":
        return EvaluatorGateResult(outcome="blocked", reason="required validation failed")
    if evidence.source not in {"request", "policy"}:
        return EvaluatorGateResult(outcome="blocked", reason="validation evidence source must be request or policy")
    if evidence.patch_digest != gate_input.patch_bundle_digest or evidence.tree_digest != gate_input.tree_digest:
        return EvaluatorGateResult(outcome="blocked", reason="validation evidence is not current for evaluated patch")
    if evidence.request_digest != gate_input.request_digest:
        return EvaluatorGateResult(outcome="blocked", reason="validation evidence request digest does not match")
    if evidence.effective_policy_digest != gate_input.effective_policy_digest:
        return EvaluatorGateResult(outcome="blocked", reason="validation evidence policy digest does not match")
    if evidence.actor_invocation_digest != gate_input.actor_invocation_digest:
        return EvaluatorGateResult(outcome="blocked", reason="validation evidence actor digest does not match")
    if digest_payload(evidence.model_dump(mode="json")) != gate_input.validation_evidence_digest:
        return EvaluatorGateResult(outcome="blocked", reason="validation evidence digest does not match")
    if evidence.network_mode != gate_input.expected_validation_network_mode:
        return EvaluatorGateResult(outcome="blocked", reason="validation evidence network mode does not match policy")
    if evidence.mutation_status != "clean":
        return EvaluatorGateResult(outcome="blocked", reason="validation mutated the target")

    return EvaluatorGateResult(outcome="pass", reason="validation evidence accepted")


def _runtime_policy_violation_reason(gate_input: EvaluatorGateInput) -> str | None:
    for ref in gate_input.runtime_evidence_refs:
        if ref.is_absolute() or ".." in ref.parts:
            return "runtime evidence ref is unsafe"
        candidate = gate_input.artifact_dir / ref
        path = candidate.resolve(strict=False)
        if not path.is_relative_to(gate_input.artifact_dir.resolve(strict=False)) or candidate.is_symlink() or not path.is_file():
            return "runtime evidence is missing or unsafe"
        try:
            payload = json.loads(path.read_text(encoding="utf-8"))
        except json.JSONDecodeError:
            return "runtime evidence is not valid JSON"
        if not isinstance(payload, dict):
            return "runtime evidence is not a mapping"
        if payload.get("blocked_category") == "policy" or isinstance(payload.get("policy_violation"), dict):
            return "runtime policy violation evidence present"
    return None
