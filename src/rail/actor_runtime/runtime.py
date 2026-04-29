from __future__ import annotations

from pathlib import Path
from typing import Literal, Protocol

import yaml
from pydantic import BaseModel, ConfigDict

from rail.artifacts import ArtifactHandle

BlockedCategory = Literal["runtime", "validation", "policy", "environment"]


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
    blocked_category: BlockedCategory | None = None


class ActorRuntime(Protocol):
    def run(self, invocation: ActorInvocation) -> ActorResult:
        ...


def build_invocation(
    handle: ArtifactHandle,
    actor: str,
    *,
    prior_outputs: dict[str, dict[str, object]] | None = None,
    evidence_refs: list[str] | None = None,
) -> ActorInvocation:
    request = _load_request_snapshot(handle)
    return ActorInvocation(
        actor=actor,
        artifact_id=handle.artifact_id,
        artifact_dir=handle.artifact_dir,
        prompt=f"Run Rail actor {actor} for task goal: {request.get('goal', '')}",
        input={
            "artifact_id": handle.artifact_id,
            "request": request,
            "prior_outputs": prior_outputs or {},
            "evidence_refs": evidence_refs or [],
        },
        policy_digest=handle.effective_policy_digest or "sha256:policy-not-yet-bound",
    )


def _load_request_snapshot(handle: ArtifactHandle) -> dict[str, object]:
    payload = yaml.safe_load((handle.artifact_dir / "request.yaml").read_text(encoding="utf-8")) or {}
    if not isinstance(payload, dict):
        raise ValueError("request snapshot must be a mapping")
    return payload
