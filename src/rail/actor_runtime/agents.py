from __future__ import annotations

import asyncio
import json
from collections.abc import Callable, Mapping
import os
from pathlib import Path
from typing import Any, Literal

from agents import Agent, RunConfig
from pydantic import BaseModel, ConfigDict, ValidationError

from rail.actor_runtime.evidence import write_runtime_evidence
from rail.actor_runtime.events import normalize_sdk_event
from rail.actor_runtime.prompts import load_actor_catalog
from rail.actor_runtime.runtime import ActorInvocation, ActorResult
from rail.auth.credentials import CredentialSource, discover_sdk_credential_sources, validate_sdk_credential_format
from rail.auth.redaction import redact_secrets
from rail.policy.schema import ActorRuntimePolicyV2


class SDKRunResult(BaseModel):
    model_config = ConfigDict(extra="forbid")

    final_output: dict[str, object]
    trace_id: str


SDKRunner = Callable[[Agent[Any], str], SDKRunResult]
CredentialPreflight = Callable[[list[CredentialSource], ActorRuntimePolicyV2], str | None]


class RuntimeReadiness(BaseModel):
    model_config = ConfigDict(extra="forbid")

    ready: bool
    reason: str
    credential_source: str | None = None
    blocked_category: Literal["environment"] | None = None


class AgentsActorRuntime:
    def __init__(
        self,
        *,
        project_root: Path,
        policy: ActorRuntimePolicyV2,
        runner: Callable[..., SDKRunResult] | None = None,
        credential_preflight: CredentialPreflight | None = None,
    ) -> None:
        self.project_root = project_root
        self.policy = policy
        self.catalog = load_actor_catalog(project_root)
        self.runner = runner or run_agent_live
        self._runner_injected = runner is not None
        self.credential_preflight = credential_preflight or validate_live_sdk_credentials
        self._readiness_cache: RuntimeReadiness | None = None

    def readiness(self) -> RuntimeReadiness:
        if self._runner_injected:
            return RuntimeReadiness(
                ready=True,
                reason="runner injected for deterministic execution",
                credential_source="injected_runner",
            )
        if self._readiness_cache is not None:
            return self._readiness_cache
        sources = discover_sdk_credential_sources()
        if not sources:
            return self._cache_readiness(
                RuntimeReadiness(
                    ready=False,
                    reason="operator SDK credential is not configured",
                    blocked_category="environment",
                )
            )
        if not _live_runtime_enabled():
            return self._cache_readiness(
                RuntimeReadiness(
                    ready=False,
                    reason="live actor runtime is not enabled",
                    credential_source=sources[0].category,
                    blocked_category="environment",
                )
            )
        if self.policy.approval_policy.mode != "never":
            return self._cache_readiness(
                RuntimeReadiness(
                    ready=False,
                    reason="live actor runtime only supports approval_policy=never",
                    credential_source=sources[0].category,
                    blocked_category="environment",
                )
            )
        preflight_failure = self.credential_preflight(sources, self.policy)
        if preflight_failure:
            return self._cache_readiness(
                RuntimeReadiness(
                    ready=False,
                    reason=preflight_failure,
                    credential_source=sources[0].category,
                    blocked_category="environment",
                )
            )
        return self._cache_readiness(
            RuntimeReadiness(
                ready=True,
                reason="operator SDK credential configured",
                credential_source=sources[0].category,
            )
        )

    def _cache_readiness(self, readiness: RuntimeReadiness) -> RuntimeReadiness:
        self._readiness_cache = readiness
        return readiness

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
                invocation.attempt_ref,
                invocation.actor,
                normalize_sdk_event(
                    {
                        "status": "interrupted",
                        "actor": invocation.actor,
                        "error": readiness.reason,
                        "credential_source": readiness.credential_source,
                        "blocked_category": readiness.blocked_category,
                    }
                ),
            )
            return ActorResult(
                status="interrupted",
                structured_output={"error": readiness.reason},
                events_ref=events_ref,
                runtime_evidence_ref=evidence_ref,
                blocked_category=readiness.blocked_category,
            )
        entry = self.catalog[invocation.actor]
        agent = self.build_agent(invocation.actor)
        prompt = (
            f"{invocation.prompt}\n\n"
            f"Policy digest: {invocation.policy_digest}\n\n"
            f"Actor input JSON:\n{json.dumps(invocation.input, sort_keys=True, ensure_ascii=False)}"
        )
        run_config: dict[str, object] = {
            "timeout_seconds": self.policy.runtime.timeout_seconds,
            "max_actor_turns": self.policy.actor_runtime.max_actor_turns,
            "approval_policy": self.policy.approval_policy.mode,
        }
        try:
            sdk_result = self.runner(agent, prompt, run_config=run_config)
            structured = entry.validate_output(sdk_result.final_output).model_dump(mode="json")
            events_ref, evidence_ref = write_runtime_evidence(
                invocation.artifact_dir,
                invocation.attempt_ref,
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
        except Exception as exc:
            events_ref, evidence_ref = write_runtime_evidence(
                invocation.artifact_dir,
                invocation.attempt_ref,
                invocation.actor,
                normalize_sdk_event({"status": "interrupted", "actor": invocation.actor, "error": str(exc)}),
            )
            message = str(redact_secrets(str(exc)))
            if isinstance(exc, ValidationError):
                message = f"validation failed: {message}"
            return ActorResult(
                status="interrupted",
                structured_output={"error": message},
                events_ref=events_ref,
                runtime_evidence_ref=evidence_ref,
                blocked_category="runtime",
            )


def validate_live_sdk_credentials(sources: list[CredentialSource], policy: ActorRuntimePolicyV2) -> str | None:
    for source in sources:
        try:
            validate_sdk_credential_format(source)
        except ValueError as exc:
            return str(exc)
    try:
        from openai import OpenAI

        source = sources[0]
        client = OpenAI(api_key=source.value) if source.value is not None else OpenAI()
        client.models.list(timeout=policy.runtime.timeout_seconds)
    except Exception:
        return "operator SDK invalid credential is configured"
    return None


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

    timeout_seconds = _run_config_int(run_config, "timeout_seconds", 180)
    max_actor_turns = _run_config_int(run_config, "max_actor_turns", 3)
    sdk_run_config = RunConfig(
        tracing_disabled=False,
        trace_include_sensitive_data=False,
        workflow_name="Rail Actor Runtime",
    )
    result = asyncio.run(
        asyncio.wait_for(
            Runner.run(agent, prompt, max_turns=max_actor_turns, run_config=sdk_run_config),
            timeout=timeout_seconds,
        )
    )
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


def _live_runtime_enabled(environ: Mapping[str, str] | None = None) -> bool:
    environ = environ or os.environ
    explicit = environ.get("RAIL_ACTOR_RUNTIME_LIVE")
    if explicit is not None and explicit.strip():
        return explicit.strip().lower() in {"1", "true", "yes", "on"}
    return bool(environ.get("OPENAI_API_KEY", "").strip())


def _run_config_int(run_config: dict[str, object], key: str, default: int) -> int:
    value = run_config.get(key, default)
    if isinstance(value, int):
        return value
    if isinstance(value, str):
        return int(value)
    raise ValueError(f"{key} must be an integer")
