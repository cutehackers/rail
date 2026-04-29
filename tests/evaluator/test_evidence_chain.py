from __future__ import annotations

from rail.artifacts.digests import digest_payload
from rail.evaluator.gate import EvaluatorGateInput
from rail.workspace.validation import ValidationEvidence


def test_digest_payload_is_stable_for_evidence_chain():
    assert digest_payload({"b": 2, "a": 1}) == digest_payload({"a": 1, "b": 2})
    assert digest_payload({"a": 1}).startswith("sha256:")


def test_evaluator_gate_input_requires_all_chain_digests(tmp_path):
    gate_input = EvaluatorGateInput(
        artifact_dir=tmp_path,
        request_digest="sha256:request",
        effective_policy_digest="sha256:policy",
        actor_invocation_digest="sha256:actor",
        patch_bundle_digest="sha256:patch",
        tree_digest="sha256:tree",
        validation_ref=None,
        validation_evidence_digest="sha256:validation",
        evaluator_input_digest="sha256:evaluator",
    )

    assert gate_input.request_digest
    assert gate_input.effective_policy_digest
    assert gate_input.actor_invocation_digest
    assert gate_input.patch_bundle_digest
    assert gate_input.tree_digest
    assert gate_input.validation_evidence_digest
    assert gate_input.evaluator_input_digest


def test_validation_evidence_records_restriction_and_mutation_status():
    evidence = ValidationEvidence(
        command="pytest",
        exit_code=0,
        status="pass",
        source="policy",
        stdout_ref="validation/stdout.txt",
        stderr_ref="validation/stderr.txt",
        patch_digest="sha256:patch",
        tree_digest="sha256:tree",
        credential_mode="minimum",
        network_mode="disabled",
        sandbox_ref="sandbox",
        mutation_status="clean",
        ref="validation/evidence.yaml",
    )

    assert evidence.credential_mode == "minimum"
    assert evidence.network_mode == "disabled"
    assert evidence.mutation_status == "clean"
