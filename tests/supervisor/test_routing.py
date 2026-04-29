from __future__ import annotations

from pathlib import Path

import yaml

import rail
from rail.supervisor.graph import SUPERVISOR_GRAPH
from rail.supervisor.router import route_next
from rail.supervisor.state import SupervisorState


def test_initial_supervisor_graph_is_deterministic():
    assert SUPERVISOR_GRAPH == ("planner", "context_builder", "critic", "generator", "executor", "evaluator")


def test_linear_routing_reaches_evaluator():
    state = SupervisorState.created("artifact-1")

    for expected in SUPERVISOR_GRAPH:
        assert state.current_actor == expected
        state = route_next(state, actor_output={})

    assert state.terminal is True
    assert state.outcome == "pass"


def test_evaluator_revise_routes_back_to_generator_with_budget():
    state = SupervisorState(artifact_id="artifact-1", current_actor="evaluator", revision_budget=1)

    routed = route_next(state, actor_output={"decision": "revise"})

    assert routed.current_actor == "generator"
    assert routed.revision_budget == 0
    assert routed.terminal is False


def test_evaluator_reject_is_terminal_failure():
    state = SupervisorState(artifact_id="artifact-1", current_actor="evaluator", revision_budget=1)

    routed = route_next(state, actor_output={"decision": "reject"})

    assert routed.terminal is True
    assert routed.outcome == "reject"


def test_supervise_api_updates_artifact_run_status(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))

    result = rail.supervise(handle)

    assert result.outcome == "pass"
    run_status = yaml.safe_load((handle.artifact_dir / "run_status.yaml").read_text(encoding="utf-8"))
    assert run_status["status"] == "terminal"
    assert run_status["outcome"] == "pass"
    assert run_status["current_actor"] == "evaluator"


def _target_repo(tmp_path: Path) -> Path:
    target = tmp_path / "target-repo"
    target.mkdir(parents=True, exist_ok=True)
    return target


def _draft(target: Path) -> dict[str, object]:
    return {
        "project_root": str(target),
        "task_type": "bug_fix",
        "goal": "Route the supervisor graph.",
        "definition_of_done": ["Supervisor reaches terminal pass."],
    }
