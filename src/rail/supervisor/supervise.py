from __future__ import annotations

from pathlib import Path

import yaml

from rail.artifacts import ArtifactHandle, bind_effective_policy, validate_artifact_handle
from rail.artifacts.digests import digest_payload
from rail.artifacts.terminal_summary import write_terminal_summary
from rail.actor_runtime.evidence import write_runtime_evidence
from rail.actor_runtime.events import normalize_sdk_event
from rail.actor_runtime.agents import AgentsActorRuntime
from rail.actor_runtime.runtime import ActorResult, ActorRuntime, build_invocation
from rail.auth.redaction import redact_secrets
from rail.evaluator.gate import EvaluatorGateInput, evaluate_gate
from rail.policy import digest_policy, load_effective_policy
from rail.policy.schema import ActorRuntimePolicyV2
from rail.supervisor.graph import SUPERVISOR_GRAPH
from rail.supervisor.router import route_next
from rail.supervisor.state import SupervisorState
from rail.workspace.apply import apply_patch_bundle
from rail.workspace.isolation import tree_digest
from rail.workspace.patch_bundle import PatchBundle, PatchValidationPolicy
from rail.workspace.validation_policy import load_policy_validation_commands
from rail.workspace.validation_runner import run_validation_commands


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
    terminal_reason: str | None = None
    blocked_category: str | None = None
    actor_outputs: dict[str, dict[str, object]] = {}
    evidence_refs: list[str] = []

    while not state.terminal:
        visited.append(state.current_actor)
        invocation = build_invocation(
            handle,
            state.current_actor,
            prior_outputs=actor_outputs,
            evidence_refs=evidence_refs,
        )
        actor_invocation_digest = digest_payload(invocation.model_dump(mode="json"))
        try:
            result = runtime.run(invocation)
        except Exception as exc:
            events_ref, evidence_ref = write_runtime_evidence(
                handle.artifact_dir,
                state.current_actor,
                normalize_sdk_event({"status": "interrupted", "actor": state.current_actor, "error": str(exc)}),
            )
            result = ActorResult(
                status="interrupted",
                structured_output={"error": str(redact_secrets(str(exc)))},
                events_ref=events_ref,
                runtime_evidence_ref=evidence_ref,
            )
        evidence_refs.extend([result.events_ref.as_posix(), result.runtime_evidence_ref.as_posix()])
        if result.status != "succeeded":
            terminal_reason = _actor_blocked_reason(state.current_actor, result)
            blocked_category = "runtime"
            state = state.finish("blocked")
            break
        actor_outputs[state.current_actor] = result.structured_output
        if state.current_actor == "generator":
            try:
                patch_digest, target_tree_digest = _apply_generator_patch(handle, policy, result)
            except (ValueError, yaml.YAMLError) as exc:
                terminal_reason = str(exc)
                blocked_category = "policy"
                state = state.finish("blocked")
                break
        if state.current_actor == "executor":
            validation_actor_invocation_digest = actor_invocation_digest
            try:
                validation_commands = load_policy_validation_commands(handle.project_root)
                validation = run_validation_commands(
                    artifact_dir=handle.artifact_dir,
                    target_root=handle.project_root,
                    commands=validation_commands,
                    patch_digest=patch_digest,
                    request_digest=handle.request_snapshot_digest,
                    effective_policy_digest=effective_policy_digest,
                    actor_invocation_digest=validation_actor_invocation_digest,
                )
            except (ValueError, yaml.YAMLError) as exc:
                terminal_reason = str(redact_secrets(str(exc)))
                blocked_category = "validation"
                state = state.finish("blocked")
                break
            target_tree_digest = validation.tree_digest
            validation_ref = validation.ref
            evidence_refs.append(validation.ref.as_posix())
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
                terminal_reason = gate.reason
                state = state.finish("pass")
            elif gate.outcome == "reject":
                terminal_reason = gate.reason
                state = state.finish("reject")
            elif gate.outcome == "revise":
                state = route_next(state, actor_output=result.structured_output)
                if state.terminal:
                    terminal_reason = "evaluator revision budget exhausted"
                    blocked_category = "runtime"
                continue
            else:
                terminal_reason = gate.reason
                blocked_category = _gate_blocked_category(gate.reason)
                state = state.finish("blocked")
            break
        state = route_next(state, actor_output=result.structured_output)

    _write_run_status(handle, state, visited, reason=terminal_reason, blocked_category=blocked_category)
    write_terminal_summary(handle)
    return state


def _write_run_status(
    handle: ArtifactHandle,
    state: SupervisorState,
    visited: list[str],
    *,
    reason: str | None = None,
    blocked_category: str | None = None,
) -> None:
    payload = {
        "schema_version": "1",
        "artifact_id": handle.artifact_id,
        "status": "terminal" if state.outcome in {"pass", "reject"} else "blocked",
        "outcome": state.outcome,
        "current_actor": SUPERVISOR_GRAPH[-1],
        "visited": visited,
    }
    if reason:
        payload["reason"] = str(redact_secrets(reason))
    if blocked_category:
        payload["blocked_category"] = blocked_category
    payload = redact_secrets(payload)
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


def _actor_blocked_reason(actor: str, result: ActorResult) -> str:
    error = result.structured_output.get("error")
    if isinstance(error, str) and error:
        return f"Actor Runtime blocked during {actor}: {redact_secrets(error)}"
    return f"Actor Runtime blocked during {actor}"


def _gate_blocked_category(reason: str) -> str:
    lowered = reason.lower()
    if "validation" in lowered:
        return "validation"
    if "policy" in lowered or "patch" in lowered:
        return "policy"
    return "runtime"


def _rail_root() -> Path:
    return Path(__file__).resolve().parents[3]
