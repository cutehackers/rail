from __future__ import annotations

from pathlib import Path

from rail.artifacts.digests import digest_payload
from rail.evaluator.gate import EvaluatorGateInput, evaluate_gate
from rail.workspace.validation import record_validation_evidence


def test_terminal_pass_requires_validation_evidence(tmp_path):
    gate_input = _gate_input(tmp_path, validation_ref=None)

    result = evaluate_gate({"decision": "pass", "findings": [], "reason_codes": [], "quality_confidence": "high"}, gate_input)

    assert result.outcome == "blocked"
    assert "validation evidence" in result.reason


def test_terminal_pass_rejects_failed_validation(tmp_path):
    evidence = record_validation_evidence(
        tmp_path, command="pytest", exit_code=1, source="request", patch_digest="sha256:patch", tree_digest="sha256:tree"
    )

    result = evaluate_gate({"decision": "pass", "findings": [], "reason_codes": [], "quality_confidence": "high"}, _gate_input(tmp_path, evidence.ref))

    assert result.outcome == "blocked"
    assert "failed" in result.reason


def test_terminal_pass_rejects_stale_validation(tmp_path):
    evidence = record_validation_evidence(
        tmp_path, command="pytest", exit_code=0, source="request", patch_digest="sha256:old", tree_digest="sha256:tree"
    )

    result = evaluate_gate({"decision": "pass", "findings": [], "reason_codes": [], "quality_confidence": "high"}, _gate_input(tmp_path, evidence.ref))

    assert result.outcome == "blocked"
    assert "current" in result.reason


def test_validation_command_must_be_request_or_policy_owned(tmp_path):
    evidence = record_validation_evidence(
        tmp_path, command="pytest", exit_code=0, source="actor", patch_digest="sha256:patch", tree_digest="sha256:tree"
    )

    result = evaluate_gate({"decision": "pass", "findings": [], "reason_codes": [], "quality_confidence": "high"}, _gate_input(tmp_path, evidence.ref))

    assert result.outcome == "blocked"
    assert "source" in result.reason


def test_evaluator_revise_and_reject_route_without_terminal_pass(tmp_path):
    gate_input = _gate_input(tmp_path, validation_ref=None)

    revise = evaluate_gate({"decision": "revise", "findings": ["fix"], "reason_codes": ["risk"], "quality_confidence": "medium"}, gate_input)
    reject = evaluate_gate({"decision": "reject", "findings": ["bad"], "reason_codes": ["risk"], "quality_confidence": "low"}, gate_input)

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

    output = {"decision": "pass", "findings": [], "reason_codes": [], "quality_confidence": "high"}
    result = evaluate_gate(output, _gate_input(tmp_path, evidence.ref, evaluator_input_digest=digest_payload(output)))

    assert result.outcome == "pass"


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
    output = {"decision": "pass", "findings": [], "reason_codes": [], "quality_confidence": "high"}

    result = evaluate_gate(output, _gate_input(tmp_path, evidence.ref, evaluator_input_digest=digest_payload(output)))

    assert result.outcome == "blocked"
    assert "request digest" in result.reason


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
    tmp_path: Path, validation_ref: Path | None, evaluator_input_digest: str = "sha256:evaluator"
) -> EvaluatorGateInput:
    return EvaluatorGateInput(
        artifact_dir=tmp_path,
        request_digest="sha256:request",
        effective_policy_digest="sha256:policy",
        actor_invocation_digest="sha256:actor",
        patch_bundle_digest="sha256:patch",
        tree_digest="sha256:tree",
        validation_ref=validation_ref,
        evaluator_input_digest=evaluator_input_digest,
    )
