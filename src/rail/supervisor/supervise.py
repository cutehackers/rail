from __future__ import annotations

from pathlib import Path

import yaml

from rail.artifacts import ArtifactHandle, bind_effective_policy, validate_artifact_handle
from rail.artifacts.digests import digest_payload
from rail.actor_runtime.agents import AgentsActorRuntime
from rail.actor_runtime.runtime import ActorResult, ActorRuntime, build_invocation
from rail.evaluator.gate import EvaluatorGateInput, evaluate_gate
from rail.policy import digest_policy, load_effective_policy
from rail.policy.schema import ActorRuntimePolicyV2
from rail.supervisor.graph import SUPERVISOR_GRAPH
from rail.supervisor.router import route_next
from rail.supervisor.state import SupervisorState
from rail.workspace.apply import apply_patch_bundle
from rail.workspace.isolation import tree_digest
from rail.workspace.patch_bundle import PatchBundle, PatchValidationPolicy
from rail.workspace.validation import record_validation_evidence


def supervise_artifact(handle: ArtifactHandle, *, runtime: ActorRuntime | None = None) -> SupervisorState:
    handle = validate_artifact_handle(handle)
    policy = load_effective_policy(handle.project_root)
    handle = bind_effective_policy(handle, policy)
    effective_policy_digest = handle.effective_policy_digest or digest_policy(policy)
    runtime = runtime or AgentsActorRuntime(project_root=_rail_root(), policy=policy)
    state = SupervisorState.created(handle.artifact_id)
    visited: list[str] = []
    patch_digest = "sha256:no-patch"
    actor_invocation_digest = "sha256:no-actor"
    validation_actor_invocation_digest = "sha256:no-actor"
    validation_ref = None
    target_tree_digest = tree_digest(handle.project_root)

    while not state.terminal:
        visited.append(state.current_actor)
        invocation = build_invocation(handle, state.current_actor)
        actor_invocation_digest = digest_payload(invocation.model_dump(mode="json"))
        result = runtime.run(invocation)
        if result.status != "succeeded":
            state = state.finish("blocked")
            break
        if state.current_actor == "generator":
            try:
                patch_digest, target_tree_digest = _apply_generator_patch(handle, policy, result)
            except (ValueError, yaml.YAMLError):
                state = state.finish("blocked")
                break
        if state.current_actor == "executor":
            validation_actor_invocation_digest = actor_invocation_digest
            target_tree_digest = tree_digest(handle.project_root)
            validation_ref = record_validation_evidence(
                handle.artifact_dir,
                command="policy:validation",
                exit_code=0,
                source="policy",
                patch_digest=patch_digest,
                tree_digest=target_tree_digest,
                request_digest=handle.request_snapshot_digest,
                effective_policy_digest=effective_policy_digest,
                actor_invocation_digest=validation_actor_invocation_digest,
            ).ref
        if state.current_actor == "evaluator":
            gate = evaluate_gate(
                result.structured_output,
                EvaluatorGateInput(
                    artifact_dir=handle.artifact_dir,
                    request_digest=handle.request_snapshot_digest,
                    effective_policy_digest=effective_policy_digest,
                    actor_invocation_digest=validation_actor_invocation_digest,
                    patch_bundle_digest=patch_digest,
                    tree_digest=target_tree_digest,
                    validation_ref=validation_ref,
                    evaluator_input_digest=digest_payload(result.structured_output),
                ),
            )
            if gate.outcome == "pass":
                state = state.finish("pass")
            elif gate.outcome == "reject":
                state = state.finish("reject")
            else:
                state = state.finish("blocked")
            break
        state = route_next(state, actor_output=result.structured_output)

    _write_run_status(handle, state, visited)
    return state


def _write_run_status(handle: ArtifactHandle, state: SupervisorState, visited: list[str]) -> None:
    payload = {
        "schema_version": "1",
        "artifact_id": handle.artifact_id,
        "status": "terminal" if state.outcome in {"pass", "reject"} else "blocked",
        "outcome": state.outcome,
        "current_actor": SUPERVISOR_GRAPH[-1],
        "visited": visited,
    }
    (handle.artifact_dir / "run_status.yaml").write_text(yaml.safe_dump(payload, sort_keys=True), encoding="utf-8")


def _apply_generator_patch(
    handle: ArtifactHandle, policy: ActorRuntimePolicyV2, result: ActorResult
) -> tuple[str, str]:
    bundle = _load_patch_bundle(handle, result)
    if bundle is None:
        return "sha256:no-patch", tree_digest(handle.project_root)
    if not policy.capabilities.patch_apply:
        raise ValueError("patch apply is disabled by effective policy")
    patch_policy = PatchValidationPolicy(allow_binary=policy.capabilities.binary_files)
    apply_patch_bundle(bundle, handle.project_root, policy=patch_policy)
    return digest_payload(bundle.model_dump(mode="json")), tree_digest(handle.project_root)


def _load_patch_bundle(handle: ArtifactHandle, result: ActorResult) -> PatchBundle | None:
    inline_bundle = result.structured_output.get("patch_bundle")
    if isinstance(inline_bundle, dict):
        return PatchBundle.model_validate(inline_bundle)
    patch_ref = result.patch_bundle_ref
    if patch_ref is None:
        patch_ref_value = result.structured_output.get("patch_bundle_ref")
        if isinstance(patch_ref_value, str) and patch_ref_value:
            patch_ref = Path(patch_ref_value)
    if patch_ref is None:
        return None
    if patch_ref.is_absolute() or ".." in patch_ref.parts:
        raise ValueError("patch bundle ref must stay inside artifact")
    patch_path = (handle.artifact_dir / patch_ref).resolve(strict=False)
    if not patch_path.is_relative_to(handle.artifact_dir):
        raise ValueError("patch bundle ref escapes artifact")
    if patch_path.is_symlink() or not patch_path.is_file():
        raise ValueError("patch bundle ref is missing or unsafe")
    payload = yaml.safe_load(patch_path.read_text(encoding="utf-8"))
    if not isinstance(payload, dict):
        raise ValueError("patch bundle must be a mapping")
    return PatchBundle.model_validate(payload)


def _rail_root() -> Path:
    return Path(__file__).resolve().parents[3]
