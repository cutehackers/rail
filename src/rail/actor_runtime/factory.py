from __future__ import annotations

from pathlib import Path

from rail.actor_runtime.agents import AgentsActorRuntime
from rail.actor_runtime.codex_vault import CodexVaultActorRuntime
from rail.actor_runtime.runtime import ActorRuntime
from rail.policy.schema import ActorRuntimePolicyV2


def build_actor_runtime(*, project_root: Path, policy: ActorRuntimePolicyV2) -> ActorRuntime:
    if policy.runtime.provider == "codex_vault":
        return CodexVaultActorRuntime(project_root=project_root, policy=policy)
    if policy.runtime.provider == "openai_agents_sdk":
        return AgentsActorRuntime(project_root=project_root, policy=policy)
    raise ValueError(f"unsupported runtime provider: {policy.runtime.provider}")
