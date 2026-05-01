from __future__ import annotations

import json
from pathlib import Path

from rail.artifacts.digests import digest_payload
from rail.evaluator.gate import EvaluatorGateInput, evaluate_gate
from rail.workspace.validation import load_validation_evidence, record_validation_evidence


def test_terminal_pass_requires_validation_evidence(tmp_path):
    gate_input = _gate_input(tmp_path, validation_ref=None)

    result = evaluate_gate(_pass_output(), gate_input)

    assert result.outcome == "blocked"
    assert "validation evidence" in result.reason


def test_terminal_pass_rejects_failed_validation(tmp_path):
    evidence = record_validation_evidence(
        tmp_path, command="pytest", exit_code=1, source="request", patch_digest="sha256:patch", tree_digest="sha256:tree"
    )

    result = evaluate_gate(_pass_output(), _gate_input(tmp_path, evidence.ref))

    assert result.outcome == "blocked"
    assert "failed" in result.reason


def test_terminal_pass_rejects_stale_validation(tmp_path):
    evidence = record_validation_evidence(
        tmp_path, command="pytest", exit_code=0, source="request", patch_digest="sha256:old", tree_digest="sha256:tree"
    )

    result = evaluate_gate(_pass_output(), _gate_input(tmp_path, evidence.ref))

    assert result.outcome == "blocked"
    assert "current" in result.reason


def test_validation_command_must_be_request_or_policy_owned(tmp_path):
    evidence = record_validation_evidence(
        tmp_path, command="pytest", exit_code=0, source="actor", patch_digest="sha256:patch", tree_digest="sha256:tree"
    )

    result = evaluate_gate(_pass_output(), _gate_input(tmp_path, evidence.ref))

    assert result.outcome == "blocked"
    assert "source" in result.reason


def test_evaluator_revise_and_reject_route_without_terminal_pass(tmp_path):
    gate_input = _gate_input(tmp_path, validation_ref=None)

    revise = evaluate_gate(
        {
            "decision": "revise",
            "evaluated_input_digest": "sha256:evaluator",
            "findings": ["fix"],
            "reason_codes": ["risk"],
            "quality_confidence": "medium",
        },
        gate_input,
    )
    reject = evaluate_gate(
        {
            "decision": "reject",
            "evaluated_input_digest": "sha256:evaluator",
            "findings": ["bad"],
            "reason_codes": ["risk"],
            "quality_confidence": "low",
        },
        gate_input,
    )

    assert revise.outcome == "revise"
    assert revise.next_actor == "generator"
    assert reject.outcome == "reject"


def test_terminal_pass_accepts_current_clean_validation(tmp_path):
    evidence = record_validation_evidence(
        tmp_path,
        command="pytest",
        exit_code=0,
        source="request",
        patch_digest="sha256:patch",
        tree_digest="sha256:tree",
        request_digest="sha256:request",
        effective_policy_digest="sha256:policy",
        actor_invocation_digest="sha256:actor",
    )

    output = _pass_output()
    result = evaluate_gate(output, _gate_input(tmp_path, evidence.ref))

    assert result.outcome == "pass"


def test_terminal_pass_rejects_policy_inconsistent_network_mode(tmp_path):
    evidence = record_validation_evidence(
        tmp_path,
        command="pytest",
        exit_code=0,
        source="request",
        patch_digest="sha256:patch",
        tree_digest="sha256:tree",
        request_digest="sha256:request",
        effective_policy_digest="sha256:policy",
        actor_invocation_digest="sha256:actor",
        network_mode="inherited",
    )

    result = evaluate_gate(_pass_output(), _gate_input(tmp_path, evidence.ref))

    assert result.outcome == "blocked"
    assert "network mode" in result.reason


def test_terminal_pass_rejects_mismatched_request_policy_actor_or_evaluator_digest(tmp_path):
    evidence = record_validation_evidence(
        tmp_path,
        command="pytest",
        exit_code=0,
        source="request",
        patch_digest="sha256:patch",
        tree_digest="sha256:tree",
        request_digest="sha256:other-request",
        effective_policy_digest="sha256:policy",
        actor_invocation_digest="sha256:actor",
    )
    output = _pass_output()

    result = evaluate_gate(output, _gate_input(tmp_path, evidence.ref))

    assert result.outcome == "blocked"
    assert "request digest" in result.reason


def test_terminal_pass_rejects_evaluator_output_not_bound_to_recorded_evaluator_input(tmp_path):
    evidence = record_validation_evidence(
        tmp_path,
        command="pytest",
        exit_code=0,
        source="request",
        patch_digest="sha256:patch",
        tree_digest="sha256:tree",
        request_digest="sha256:request",
        effective_policy_digest="sha256:policy",
        actor_invocation_digest="sha256:actor",
    )
    output = {"decision": "pass", "findings": [], "reason_codes": [], "quality_confidence": "high"}

    result = evaluate_gate(output, _gate_input(tmp_path, evidence.ref, evaluator_input_digest=digest_payload(output)))

    assert result.outcome == "blocked"
    assert "evaluator input digest" in result.reason


def test_revise_and_reject_require_evaluator_input_digest_binding(tmp_path):
    gate_input = _gate_input(tmp_path, validation_ref=None)

    revise = evaluate_gate(
        {
            "decision": "revise",
            "evaluated_input_digest": "sha256:wrong",
            "findings": ["fix"],
            "reason_codes": ["risk"],
            "quality_confidence": "medium",
        },
        gate_input,
    )
    reject = evaluate_gate(
        {
            "decision": "reject",
            "evaluated_input_digest": "sha256:wrong",
            "findings": ["bad"],
            "reason_codes": ["risk"],
            "quality_confidence": "low",
        },
        gate_input,
    )

    assert revise.outcome == "blocked"
    assert reject.outcome == "blocked"
    assert "evaluator input digest" in revise.reason
    assert "evaluator input digest" in reject.reason


def test_terminal_pass_rejects_mismatched_validation_evidence_digest(tmp_path):
    evidence = record_validation_evidence(
        tmp_path,
        command="pytest",
        exit_code=0,
        source="request",
        patch_digest="sha256:patch",
        tree_digest="sha256:tree",
        request_digest="sha256:request",
        effective_policy_digest="sha256:policy",
        actor_invocation_digest="sha256:actor",
    )

    result = evaluate_gate(
        _pass_output(),
        _gate_input(tmp_path, evidence.ref, validation_evidence_digest="sha256:wrong-validation"),
    )

    assert result.outcome == "blocked"
    assert "validation evidence digest" in result.reason


def test_terminal_pass_rejects_runtime_policy_violation_evidence(tmp_path):
    evidence = record_validation_evidence(
        tmp_path,
        command="pytest",
        exit_code=0,
        source="request",
        patch_digest="sha256:patch",
        tree_digest="sha256:tree",
        request_digest="sha256:request",
        effective_policy_digest="sha256:policy",
        actor_invocation_digest="sha256:actor",
    )
    runtime_ref = Path("runs/planner.runtime_evidence.json")
    (tmp_path / runtime_ref.parent).mkdir(parents=True, exist_ok=True)
    (tmp_path / runtime_ref).write_text(
        json.dumps({"status": "succeeded", "policy_violation": {"reason": "MCP invocation is not allowed"}}),
        encoding="utf-8",
    )

    result = evaluate_gate(
        _pass_output(),
        _gate_input(tmp_path, evidence.ref, runtime_evidence_refs=[runtime_ref]),
    )

    assert result.outcome == "blocked"
    assert "runtime policy violation" in result.reason


def test_validation_evidence_redacts_logs_before_persisting(tmp_path):
    evidence = record_validation_evidence(
        tmp_path,
        command="pytest",
        exit_code=0,
        source="request",
        patch_digest="sha256:patch",
        tree_digest="sha256:tree",
        stdout="OPENAI_API_KEY=sk-secret-value",
    )

    assert "sk-secret-value" not in (tmp_path / evidence.stdout_ref).read_text(encoding="utf-8")


def _gate_input(
    tmp_path: Path,
    validation_ref: Path | None,
    evaluator_input_digest: str = "sha256:evaluator",
    validation_evidence_digest: str | None = None,
    runtime_evidence_refs: list[Path] | None = None,
) -> EvaluatorGateInput:
    if validation_evidence_digest is None:
        validation_evidence_digest = "sha256:no-validation"
        if validation_ref is not None:
            evidence = load_validation_evidence(tmp_path, validation_ref)
            validation_evidence_digest = digest_payload(evidence.model_dump(mode="json"))
    return EvaluatorGateInput(
        artifact_dir=tmp_path,
        request_digest="sha256:request",
        effective_policy_digest="sha256:policy",
        actor_invocation_digest="sha256:actor",
        patch_bundle_digest="sha256:patch",
        tree_digest="sha256:tree",
        validation_ref=validation_ref,
        validation_evidence_digest=validation_evidence_digest,
        evaluator_input_digest=evaluator_input_digest,
        runtime_evidence_refs=runtime_evidence_refs or [],
    )


def _pass_output(evaluator_input_digest: str = "sha256:evaluator") -> dict[str, object]:
    return {
        "decision": "pass",
        "findings": [],
        "reason_codes": [],
        "quality_confidence": "high",
        "evaluated_input_digest": evaluator_input_digest,
    }
