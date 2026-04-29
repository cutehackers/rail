from __future__ import annotations

from collections.abc import Callable
from pathlib import Path
from typing import Any

from agents import Agent
from pydantic import BaseModel, ConfigDict, ValidationError

from rail.actor_runtime.evidence import write_runtime_evidence
from rail.actor_runtime.events import normalize_sdk_event
from rail.actor_runtime.prompts import load_actor_catalog
from rail.actor_runtime.runtime import ActorInvocation, ActorResult
from rail.policy.schema import ActorRuntimePolicyV2


class SDKRunResult(BaseModel):
    model_config = ConfigDict(extra="forbid")

    final_output: dict[str, object]
    trace_id: str


SDKRunner = Callable[[Agent[Any], str], SDKRunResult]


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
        self.runner = runner or _offline_runner_not_configured

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
        entry = self.catalog[invocation.actor]
        agent = self.build_agent(invocation.actor)
        prompt = f"{invocation.prompt}\n\nPolicy digest: {invocation.policy_digest}"
        run_config = {
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


def build_sdk_tools(policy: ActorRuntimePolicyV2) -> list[object]:
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


def _offline_runner_not_configured(_agent: Agent[Any], _prompt: str, *, run_config: dict[str, object]) -> SDKRunResult:
    raise RuntimeError("Agents SDK runner is not configured for live execution")
