from __future__ import annotations

from collections.abc import Callable
import os
from pathlib import Path
from typing import Any

from agents import Agent, RunConfig
from pydantic import BaseModel, ConfigDict, ValidationError

from rail.actor_runtime.evidence import write_runtime_evidence
from rail.actor_runtime.events import normalize_sdk_event
from rail.actor_runtime.prompts import load_actor_catalog
from rail.actor_runtime.runtime import ActorInvocation, ActorResult
from rail.auth.credentials import discover_sdk_credential_sources
from rail.policy.schema import ActorRuntimePolicyV2


class SDKRunResult(BaseModel):
    model_config = ConfigDict(extra="forbid")

    final_output: dict[str, object]
    trace_id: str


SDKRunner = Callable[[Agent[Any], str], SDKRunResult]


class RuntimeReadiness(BaseModel):
    model_config = ConfigDict(extra="forbid")

    ready: bool
    reason: str
    credential_source: str | None = None


class AgentsActorRuntime:
    def __init__(
        self,
        *,
        project_root: Path,
        policy: ActorRuntimePolicyV2,
        runner: Callable[..., SDKRunResult] | None = None,
    ) -> None:
        self.project_root = project_root
        self.policy = policy
        self.catalog = load_actor_catalog(project_root)
        self.runner = runner or run_agent_live
        self._runner_injected = runner is not None

    def readiness(self) -> RuntimeReadiness:
        if self._runner_injected:
            return RuntimeReadiness(
                ready=True,
                reason="runner injected for deterministic execution",
                credential_source="injected_runner",
            )
        sources = discover_sdk_credential_sources()
        if not sources:
            return RuntimeReadiness(ready=False, reason="operator SDK credential is not configured")
        if not _live_runtime_enabled():
            return RuntimeReadiness(ready=False, reason="live actor runtime is not enabled")
        return RuntimeReadiness(
            ready=True,
            reason="operator SDK credential configured",
            credential_source=sources[0].category,
        )

    def build_agent(self, actor: str) -> Agent[None]:
        entry = self.catalog[actor]
        return Agent(
            name=f"rail-{actor}",
            instructions=entry.prompt,
            model=self.policy.runtime.model,
            tools=build_sdk_tools(self.policy),
            output_type=entry.output_model,
        )

    def run(self, invocation: ActorInvocation) -> ActorResult:
        readiness = self.readiness()
        if not readiness.ready:
            events_ref, evidence_ref = write_runtime_evidence(
                invocation.artifact_dir,
                invocation.actor,
                normalize_sdk_event(
                    {
                        "status": "interrupted",
                        "actor": invocation.actor,
                        "error": readiness.reason,
                        "credential_source": readiness.credential_source,
                    }
                ),
            )
            return ActorResult(
                status="interrupted",
                structured_output={"error": readiness.reason},
                events_ref=events_ref,
                runtime_evidence_ref=evidence_ref,
            )
        entry = self.catalog[invocation.actor]
        agent = self.build_agent(invocation.actor)
        prompt = f"{invocation.prompt}\n\nPolicy digest: {invocation.policy_digest}"
        run_config: dict[str, object] = {
            "timeout_seconds": self.policy.runtime.timeout_seconds,
            "approval_policy": self.policy.approval_policy.mode,
        }
        try:
            sdk_result = self.runner(agent, prompt, run_config=run_config)
            structured = entry.validate_output(sdk_result.final_output).model_dump(mode="json")
            events_ref, evidence_ref = write_runtime_evidence(
                invocation.artifact_dir,
                invocation.actor,
                normalize_sdk_event(
                    {
                        "status": "succeeded",
                        "actor": invocation.actor,
                        "trace_id": sdk_result.trace_id,
                        "structured_output": structured,
                    }
                ),
            )
            return ActorResult(
                status="succeeded",
                structured_output=structured,
                events_ref=events_ref,
                runtime_evidence_ref=evidence_ref,
                patch_bundle_ref=Path(structured["patch_bundle_ref"]) if structured.get("patch_bundle_ref") else None,
            )
        except (RuntimeError, ValidationError, ValueError) as exc:
            events_ref, evidence_ref = write_runtime_evidence(
                invocation.artifact_dir,
                invocation.actor,
                normalize_sdk_event({"status": "interrupted", "actor": invocation.actor, "error": str(exc)}),
            )
            message = str(exc)
            if isinstance(exc, ValidationError):
                message = f"validation failed: {message}"
            return ActorResult(
                status="interrupted",
                structured_output={"error": message},
                events_ref=events_ref,
                runtime_evidence_ref=evidence_ref,
            )


def build_sdk_tools(policy: ActorRuntimePolicyV2) -> list[Any]:
    if not any(
        (
            policy.tools.shell.enabled,
            policy.tools.filesystem.enabled,
            policy.tools.network.enabled,
            policy.tools.mcp.enabled,
        )
    ):
        return []
    raise ValueError("host tools require explicit implementation before enabling")


def run_agent_live(agent: Agent[Any], prompt: str, *, run_config: dict[str, object]) -> SDKRunResult:
    from agents import Runner

    sdk_run_config = RunConfig(
        tracing_disabled=False,
        trace_include_sensitive_data=False,
        workflow_name="Rail Actor Runtime",
    )
    result = Runner.run_sync(agent, prompt, run_config=sdk_run_config)
    output = _structured_output(result)
    trace_id = str(getattr(result, "trace_id", None) or "trace-unavailable")
    return SDKRunResult(final_output=output, trace_id=trace_id)


def _structured_output(result: Any) -> dict[str, object]:
    output = getattr(result, "final_output", None)
    if isinstance(output, BaseModel):
        return output.model_dump(mode="json")
    if isinstance(output, dict):
        return output
    raise ValueError("Agents SDK result did not include structured output")


def _live_runtime_enabled() -> bool:
    return os.environ.get("RAIL_ACTOR_RUNTIME_LIVE", "").strip().lower() in {"1", "true", "yes", "on"}
