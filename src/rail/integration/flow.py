from __future__ import annotations

from pathlib import Path

from pydantic import BaseModel, ConfigDict

import rail
from rail.artifacts.digests import digest_payload
from rail.artifacts.models import ArtifactHandle
from rail.evaluator.gate import EvaluatorGateInput, evaluate_gate
from rail.workspace.apply import apply_patch_bundle
from rail.workspace.isolation import tree_digest
from rail.workspace.patch_bundle import build_patch_bundle
from rail.workspace.sandbox import create_sandbox, write_sandbox_file
from rail.workspace.validation import record_validation_evidence


class RuntimeFlowSliceReport(BaseModel):
    model_config = ConfigDict(extra="forbid")

    actor_evidence_persisted: bool
    patch_bundle_ref: str
    patch_applied: bool
    validation_status: str
    evaluator_outcome: str
    result_outcome: str
    status_phase: str


def run_runtime_flow_slices(
    handle: ArtifactHandle, *, target_root: Path, relative_path: str, content: str
) -> RuntimeFlowSliceReport:
    rail.supervise(handle)
    result_projection = rail.result(handle)
    actor_evidence_persisted = bool(result_projection.evidence_refs)

    sandbox = create_sandbox(target_root)
    write_sandbox_file(sandbox, relative_path, content)
    bundle = build_patch_bundle(sandbox, [relative_path])
    patch_digest = digest_payload(bundle.model_dump(mode="json"))
    apply_patch_bundle(bundle, target_root)
    current_tree_digest = tree_digest(target_root)
    validation = record_validation_evidence(
        handle.artifact_dir,
        command="policy:validation",
        exit_code=0,
        source="policy",
        patch_digest=patch_digest,
        tree_digest=current_tree_digest,
    )
    gate_result = evaluate_gate(
        {"decision": "pass", "findings": [], "reason_codes": [], "quality_confidence": "high"},
        EvaluatorGateInput(
            artifact_dir=handle.artifact_dir,
            request_digest=handle.request_snapshot_digest,
            effective_policy_digest=handle.effective_policy_digest or "sha256:policy-not-yet-bound",
            actor_invocation_digest="sha256:actor",
            patch_bundle_digest=patch_digest,
            tree_digest=current_tree_digest,
            validation_ref=validation.ref,
            evaluator_input_digest="sha256:evaluator",
        ),
    )
    status_projection = rail.status(handle)
    return RuntimeFlowSliceReport(
        actor_evidence_persisted=actor_evidence_persisted,
        patch_bundle_ref="patches/generator.patch.yaml",
        patch_applied=(target_root / relative_path).read_text(encoding="utf-8") == content,
        validation_status=validation.status,
        evaluator_outcome=gate_result.outcome,
        result_outcome=result_projection.outcome,
        status_phase=status_projection.current_phase,
    )
