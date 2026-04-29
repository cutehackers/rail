from __future__ import annotations

import os
from pathlib import Path

import pytest

import rail
from rail.actor_runtime.agents import AgentsActorRuntime
from rail.actor_runtime.runtime import build_invocation
from rail.policy import load_effective_policy


def test_optional_live_sdk_smoke_runs_planner_actor(tmp_path):
    if os.environ.get("RAIL_ACTOR_RUNTIME_LIVE_SMOKE") != "1":
        pytest.skip("set RAIL_ACTOR_RUNTIME_LIVE_SMOKE=1 to run the optional live SDK smoke")
    if not os.environ.get("OPENAI_API_KEY"):
        pytest.fail("OPENAI_API_KEY is required for optional live SDK smoke")
    os.environ["RAIL_ACTOR_RUNTIME_LIVE"] = "1"

    target = tmp_path / "target"
    target.mkdir()
    handle = rail.start_task(
        {
            "project_root": str(target),
            "task_type": "bug_fix",
            "goal": "Run optional live planner smoke.",
            "definition_of_done": ["Planner returns structured output."],
        }
    )
    runtime = AgentsActorRuntime(project_root=Path("."), policy=load_effective_policy(target))

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "succeeded"
    assert "summary" in result.structured_output
