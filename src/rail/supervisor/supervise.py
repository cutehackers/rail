from __future__ import annotations

import yaml

from rail.artifacts import ArtifactHandle, validate_artifact_handle
from rail.actor_runtime.runtime import ActorRuntime, FakeActorRuntime, build_invocation
from rail.supervisor.graph import SUPERVISOR_GRAPH
from rail.supervisor.router import route_next
from rail.supervisor.state import SupervisorState


def supervise_artifact(handle: ArtifactHandle, *, runtime: ActorRuntime | None = None) -> SupervisorState:
    handle = validate_artifact_handle(handle)
    runtime = runtime or FakeActorRuntime()
    state = SupervisorState.created(handle.artifact_id)
    visited: list[str] = []

    while not state.terminal:
        visited.append(state.current_actor)
        result = runtime.run(build_invocation(handle, state.current_actor))
        state = route_next(state, actor_output=result.structured_output)

    _write_run_status(handle, state, visited)
    return state


def _write_run_status(handle: ArtifactHandle, state: SupervisorState, visited: list[str]) -> None:
    payload = {
        "schema_version": "1",
        "artifact_id": handle.artifact_id,
        "status": "terminal",
        "outcome": state.outcome,
        "current_actor": SUPERVISOR_GRAPH[-1],
        "visited": visited,
    }
    (handle.artifact_dir / "run_status.yaml").write_text(yaml.safe_dump(payload, sort_keys=True), encoding="utf-8")
