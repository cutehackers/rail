from __future__ import annotations

from pathlib import Path
from typing import Protocol, Literal

import yaml
from pydantic import BaseModel, ConfigDict

from rail.actor_runtime.evidence import write_runtime_evidence
from rail.actor_runtime.schemas import fake_actor_output
from rail.artifacts import ArtifactHandle
from rail.workspace.isolation import tree_digest


class ActorInvocation(BaseModel):
    model_config = ConfigDict(extra="forbid")

    actor: str
    artifact_id: str
    artifact_dir: Path
    prompt: str
    input: dict[str, object]
    policy_digest: str


class ActorResult(BaseModel):
    model_config = ConfigDict(extra="forbid")

    status: Literal["succeeded", "failed", "interrupted"]
    structured_output: dict[str, object]
    events_ref: Path
    runtime_evidence_ref: Path
    patch_bundle_ref: Path | None = None


class ActorRuntime(Protocol):
    def run(self, invocation: ActorInvocation) -> ActorResult:
        ...


class FakeActorRuntime:
    def run(self, invocation: ActorInvocation) -> ActorResult:
        output = fake_actor_output(invocation.actor)
        patch_value = output.get("patch_bundle_ref")
        patch_ref = Path(patch_value) if isinstance(patch_value, str) and invocation.actor == "generator" else None
        if patch_ref is not None:
            _write_noop_patch_bundle(invocation, patch_ref)
        events_ref, evidence_ref = write_runtime_evidence(
            invocation.artifact_dir,
            invocation.actor,
            {
                "status": "succeeded",
                "actor": invocation.actor,
                "artifact_id": invocation.artifact_id,
                "policy_digest": invocation.policy_digest,
                "structured_output": output,
            },
        )
        return ActorResult(
            status="succeeded",
            structured_output=output,
            events_ref=events_ref,
            runtime_evidence_ref=evidence_ref,
            patch_bundle_ref=patch_ref,
        )


def build_invocation(handle: ArtifactHandle, actor: str) -> ActorInvocation:
    return ActorInvocation(
        actor=actor,
        artifact_id=handle.artifact_id,
        artifact_dir=handle.artifact_dir,
        prompt=f"Run Rail actor {actor}.",
        input={"artifact_id": handle.artifact_id},
        policy_digest=handle.effective_policy_digest or "sha256:policy-not-yet-bound",
    )


def _write_noop_patch_bundle(invocation: ActorInvocation, patch_ref: Path) -> None:
    if patch_ref.is_absolute() or ".." in patch_ref.parts:
        return
    request = yaml.safe_load((invocation.artifact_dir / "request.yaml").read_text(encoding="utf-8"))
    target_root = Path(str(request["project_root"]))
    patch_path = invocation.artifact_dir / patch_ref
    patch_path.parent.mkdir(parents=True, exist_ok=True)
    patch_path.write_text(
        yaml.safe_dump({"schema_version": "1", "base_tree_digest": tree_digest(target_root), "operations": []}),
        encoding="utf-8",
    )
