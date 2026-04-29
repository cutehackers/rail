from __future__ import annotations

from pathlib import Path

from rail.actor_runtime.agents import AgentsActorRuntime, SDKRunResult
from rail.actor_runtime.schemas import fake_actor_output
from rail.policy import load_effective_policy
from rail.workspace.isolation import tree_digest


def scripted_agents_runtime(
    target_root: Path, *, patch_path: str | None = None, patch_content: str | None = None
) -> AgentsActorRuntime:
    def runner(agent, _prompt: str, *, run_config: dict[str, object]) -> SDKRunResult:
        actor = agent.name.removeprefix("rail-")
        output = fake_actor_output(actor)
        if actor == "generator":
            operations = []
            if patch_path is not None:
                operations.append({"op": "write", "path": patch_path, "content": patch_content or ""})
            output["patch_bundle"] = {
                "schema_version": "1",
                "base_tree_digest": tree_digest(target_root),
                "operations": operations,
            }
        return SDKRunResult(final_output=output, trace_id=f"trace-{actor}")

    return AgentsActorRuntime(project_root=Path("."), policy=load_effective_policy(target_root), runner=runner)
