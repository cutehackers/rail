from __future__ import annotations

import os
from pathlib import Path

import pytest

import rail
from rail.actor_runtime.codex_vault import CodexVaultActorRuntime
from rail.actor_runtime.runtime import build_invocation
from rail.policy import load_effective_policy

pytestmark = pytest.mark.skipif(
    os.environ.get("RAIL_CODEX_VAULT_LIVE_SMOKE") != "1",
    reason="codex_vault live smoke is opt-in",
)


def test_optional_codex_vault_live_smoke_runs_planner_actor(tmp_path):
    target = tmp_path / "target"
    target.mkdir()
    handle = rail.start_task(
        {
            "project_root": str(target),
            "task_type": "bug_fix",
            "goal": "Run optional Codex Vault planner smoke without mutating target.",
            "definition_of_done": ["Planner returns structured output."],
        }
    )
    runtime = CodexVaultActorRuntime(project_root=Path("."), policy=load_effective_policy(target))

    before = sorted(path.relative_to(target) for path in target.rglob("*"))
    result = runtime.run(build_invocation(handle, "planner"))
    after = sorted(path.relative_to(target) for path in target.rglob("*"))

    assert result.status == "succeeded"
    assert "summary" in result.structured_output
    assert after == before
