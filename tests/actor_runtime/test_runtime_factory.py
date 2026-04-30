from __future__ import annotations

from pathlib import Path

import pytest

from rail.actor_runtime.agents import AgentsActorRuntime
from rail.actor_runtime.factory import build_actor_runtime
from rail.policy import load_effective_policy
from rail.policy.schema import ActorRuntimePolicyV2


def test_factory_builds_codex_vault_for_default_policy(tmp_path):
    runtime = build_actor_runtime(project_root=Path("."), policy=load_effective_policy(tmp_path))

    assert runtime.__class__.__name__ == "CodexVaultActorRuntime"


def test_factory_builds_agents_runtime_when_policy_selects_sdk(tmp_path):
    policy = _sdk_policy(tmp_path)

    runtime = build_actor_runtime(project_root=Path("."), policy=policy)

    assert isinstance(runtime, AgentsActorRuntime)


def test_factory_rejects_unknown_provider_shape(tmp_path):
    policy = load_effective_policy(tmp_path)
    policy.runtime.provider = "unknown"

    with pytest.raises(ValueError, match="unsupported runtime provider: unknown"):
        build_actor_runtime(project_root=Path("."), policy=policy)


def _sdk_policy(project_root: Path) -> ActorRuntimePolicyV2:
    data = load_effective_policy(project_root).model_dump(mode="json")
    data["runtime"]["provider"] = "openai_agents_sdk"
    data["tools"]["shell"]["enabled"] = False
    data["tools"]["shell"]["allowlist"] = []
    return ActorRuntimePolicyV2.model_validate(data)
