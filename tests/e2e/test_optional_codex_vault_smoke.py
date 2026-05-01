from __future__ import annotations

import os
import json
from pathlib import Path

import pytest

import rail
from rail.artifacts.run_attempts import allocate_run_attempt
from rail.actor_runtime.codex_vault import CodexVaultActorRuntime
from rail.actor_runtime.runtime import build_invocation
from rail.policy import load_effective_policy
from rail.workspace.isolation import target_mutation_digest

pytestmark = pytest.mark.skipif(
    os.environ.get("RAIL_CODEX_VAULT_LIVE_SMOKE") != "1",
    reason="codex_vault live smoke is opt-in",
)


def test_optional_codex_vault_live_smoke_runs_planner_actor_without_openai_api_key(tmp_path, monkeypatch):
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
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

    before = target_mutation_digest(target)
    result = runtime.run(build_invocation(handle, "planner", attempt_ref=allocate_run_attempt(handle.artifact_dir)))
    after = target_mutation_digest(target)

    assert result.status == "succeeded"
    assert result.events_ref.as_posix().startswith("runs/attempt-0001/")
    assert result.runtime_evidence_ref.as_posix().startswith("runs/attempt-0001/")
    assert "summary" in result.structured_output
    assert after == before
    evidence = json.loads((handle.artifact_dir / result.runtime_evidence_ref).read_text(encoding="utf-8"))
    assert evidence["provider"] == "codex_vault"
    assert "policy_violation" not in evidence
    assert "OPENAI_API_KEY" not in json.dumps(evidence)
