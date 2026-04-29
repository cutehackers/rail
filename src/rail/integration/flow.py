from __future__ import annotations

from pathlib import Path

from pydantic import BaseModel, ConfigDict

import rail
from rail.actor_runtime.evidence import write_runtime_evidence
from rail.actor_runtime.runtime import ActorInvocation, ActorResult
from rail.actor_runtime.schemas import fake_actor_output
from rail.artifacts.models import ArtifactHandle
from rail.workspace.isolation import tree_digest
from rail.workspace.validation import load_validation_evidence


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
    runtime = _InlinePatchRuntime(target_root=target_root, relative_path=relative_path, content=content)
    rail.supervise(handle, runtime=runtime)
    result_projection = rail.result(handle)
    status_projection = rail.status(handle)
    validation_ref = Path("validation/evidence.yaml")
    validation_status = (
        load_validation_evidence(handle.artifact_dir, validation_ref).status
        if (handle.artifact_dir / validation_ref).is_file()
        else "missing"
    )

    return RuntimeFlowSliceReport(
        actor_evidence_persisted=bool(result_projection.evidence_refs),
        patch_bundle_ref="inline",
        patch_applied=(target_root / relative_path).read_text(encoding="utf-8") == content,
        validation_status=validation_status,
        evaluator_outcome=result_projection.outcome,
        result_outcome=result_projection.outcome,
        status_phase=status_projection.current_phase,
    )


class _InlinePatchRuntime:
    def __init__(self, *, target_root: Path, relative_path: str, content: str) -> None:
        self.target_root = target_root
        self.relative_path = relative_path
        self.content = content

    def run(self, invocation: ActorInvocation) -> ActorResult:
        output = fake_actor_output(invocation.actor)
        if invocation.actor == "evaluator":
            evaluator_input_digest = invocation.input.get("evaluator_input_digest")
            if isinstance(evaluator_input_digest, str):
                output["evaluated_input_digest"] = evaluator_input_digest
        if invocation.actor == "generator":
            output["changed_files"] = [self.relative_path]
            output["patch_bundle"] = {
                "schema_version": "1",
                "base_tree_digest": tree_digest(self.target_root),
                "operations": [{"op": "write", "path": self.relative_path, "content": self.content}],
            }
        events_ref, evidence_ref = write_runtime_evidence(
            invocation.artifact_dir,
            invocation.actor,
            {
                "status": "succeeded",
                "actor": invocation.actor,
                "artifact_id": invocation.artifact_id,
                "structured_output": output,
            },
        )
        return ActorResult(
            status="succeeded",
            structured_output=output,
            events_ref=events_ref,
            runtime_evidence_ref=evidence_ref,
        )
