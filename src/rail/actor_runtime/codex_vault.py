from __future__ import annotations

from pathlib import Path

from rail.actor_runtime.evidence import write_runtime_evidence
from rail.actor_runtime.events import normalize_sdk_event
from rail.actor_runtime.runtime import ActorInvocation, ActorResult
from rail.policy.schema import ActorRuntimePolicyV2


class CodexVaultActorRuntime:
    def __init__(self, *, project_root: Path, policy: ActorRuntimePolicyV2) -> None:
        self.project_root = project_root
        self.policy = policy

    def run(self, invocation: ActorInvocation) -> ActorResult:
        reason = "Codex Vault Actor Runtime environment readiness is not implemented; credential and command checks are pending"
        events_ref, evidence_ref = write_runtime_evidence(
            invocation.artifact_dir,
            invocation.actor,
            normalize_sdk_event(
                {
                    "status": "interrupted",
                    "actor": invocation.actor,
                    "error": reason,
                    "blocked_category": "environment",
                    "runtime_provider": self.policy.runtime.provider,
                    "runtime_project_root": self.project_root.as_posix(),
                    "target_root": invocation.target_root.as_posix(),
                }
            ),
        )
        return ActorResult(
            status="interrupted",
            structured_output={"error": reason},
            events_ref=events_ref,
            runtime_evidence_ref=evidence_ref,
            blocked_category="environment",
        )
