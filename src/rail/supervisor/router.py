from __future__ import annotations

from rail.supervisor.graph import SUPERVISOR_GRAPH
from rail.supervisor.state import SupervisorState


def route_next(state: SupervisorState, actor_output: dict[str, object]) -> SupervisorState:
    if state.current_actor == "evaluator":
        decision = actor_output.get("decision", "pass")
        if decision == "pass":
            return state.finish("pass")
        if decision == "reject" or state.revision_budget <= 0:
            return state.finish("reject")
        return state.model_copy(update={"current_actor": "generator", "revision_budget": state.revision_budget - 1})

    next_index = SUPERVISOR_GRAPH.index(state.current_actor) + 1
    return state.model_copy(update={"current_actor": SUPERVISOR_GRAPH[next_index]})
