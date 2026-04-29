from __future__ import annotations

from pathlib import Path
from typing import Literal, Protocol

from pydantic import BaseModel, ConfigDict

from rail.artifacts import ArtifactHandle


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


def build_invocation(handle: ArtifactHandle, actor: str) -> ActorInvocation:
    return ActorInvocation(
        actor=actor,
        artifact_id=handle.artifact_id,
        artifact_dir=handle.artifact_dir,
        prompt=f"Run Rail actor {actor}.",
        input={"artifact_id": handle.artifact_id},
        policy_digest=handle.effective_policy_digest or "sha256:policy-not-yet-bound",
    )
