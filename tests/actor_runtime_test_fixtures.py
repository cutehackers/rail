from __future__ import annotations

from pathlib import Path

import yaml

from rail.actor_runtime.evidence import write_runtime_evidence
from rail.actor_runtime.agents import AgentsActorRuntime, SDKRunResult
from rail.actor_runtime.runtime import ActorInvocation, ActorResult
from rail.actor_runtime.schemas import fake_actor_output
from rail.policy import load_effective_policy
from rail.workspace.isolation import tree_digest


def scripted_agents_runtime(
    target_root: Path, *, patch_path: str | None = None, patch_content: str | None = None
) -> AgentsActorRuntime:
    _ensure_validation_policy(target_root)

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


class FakeActorRuntime:
    def run(self, invocation: ActorInvocation) -> ActorResult:
        output = fake_actor_output(invocation.actor)
        patch_value = output.get("patch_bundle_ref")
        patch_ref = Path(patch_value) if isinstance(patch_value, str) and invocation.actor == "generator" else None
        if patch_ref is not None:
            _write_noop_patch_bundle(invocation, patch_ref)
        events_ref, evidence_ref = write_runtime_evidence(
            invocation.artifact_dir,
            invocation.actor,
            {
                "status": "succeeded",
                "actor": invocation.actor,
                "artifact_id": invocation.artifact_id,
                "policy_digest": invocation.policy_digest,
                "structured_output": output,
            },
        )
        return ActorResult(
            status="succeeded",
            structured_output=output,
            events_ref=events_ref,
            runtime_evidence_ref=evidence_ref,
            patch_bundle_ref=patch_ref,
        )


def _write_noop_patch_bundle(invocation: ActorInvocation, patch_ref: Path) -> None:
    if patch_ref.is_absolute() or ".." in patch_ref.parts:
        return
    request = yaml.safe_load((invocation.artifact_dir / "request.yaml").read_text(encoding="utf-8"))
    target_root = Path(str(request["project_root"]))
    patch_path = invocation.artifact_dir / patch_ref
    patch_path.parent.mkdir(parents=True, exist_ok=True)
    patch_path.write_text(
        yaml.safe_dump({"schema_version": "1", "base_tree_digest": tree_digest(target_root), "operations": []}),
        encoding="utf-8",
    )


def _ensure_validation_policy(target_root: Path) -> None:
    policy = target_root / ".harness" / "supervisor" / "execution_policy.yaml"
    policy.parent.mkdir(parents=True, exist_ok=True)
    if policy.is_file():
        return
    policy.write_text(
        "version: 2\nvalidation:\n  commands:\n    - python -c \"import pathlib; assert pathlib.Path('.').exists()\"\n",
        encoding="utf-8",
    )
