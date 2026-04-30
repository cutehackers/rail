from __future__ import annotations

from pathlib import Path

import rail
from rail.actor_runtime.agents import (
    AgentsActorRuntime,
    SDKRunResult,
    build_sdk_tools,
    run_agent_live,
    validate_live_sdk_credentials,
)
from rail.actor_runtime.runtime import build_invocation
from rail.auth.credentials import CredentialSource
from rail.policy import load_effective_policy
from pydantic import BaseModel


def test_default_runner_requires_ready_credentials(tmp_path, monkeypatch):
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    monkeypatch.delenv("RAIL_ACTOR_RUNTIME_LIVE", raising=False)

    runtime = AgentsActorRuntime(project_root=Path("."), policy=load_effective_policy(tmp_path))

    readiness = runtime.readiness()
    assert readiness.ready is False
    assert "credential" in readiness.reason


def test_injected_runner_is_ready_without_live_credentials(tmp_path, monkeypatch):
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    monkeypatch.delenv("RAIL_ACTOR_RUNTIME_LIVE", raising=False)

    def runner(_agent, _prompt, *, run_config):
        return SDKRunResult(
            final_output={
                "summary": "Plan",
                "likely_files": ["src/rail/api.py"],
                "substeps": ["Do work"],
                "risks": [],
                "acceptance_criteria_refined": ["Pass tests"],
            },
            trace_id="trace-ready",
        )

    runtime = AgentsActorRuntime(project_root=Path("."), policy=load_effective_policy(tmp_path), runner=runner)

    readiness = runtime.readiness()
    assert readiness.ready is True
    assert readiness.credential_source == "injected_runner"


def test_live_runner_is_ready_when_operator_credential_is_configured(tmp_path, monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test-secret")
    monkeypatch.setenv("RAIL_ACTOR_RUNTIME_LIVE", "1")

    runtime = AgentsActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(tmp_path),
        credential_preflight=lambda _sources, _policy: None,
    )

    readiness = runtime.readiness()
    assert readiness.ready is True
    assert readiness.credential_source == "operator_env"


def test_default_runner_auto_enables_live_with_operator_credential(tmp_path, monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test-secret")
    monkeypatch.delenv("RAIL_ACTOR_RUNTIME_LIVE", raising=False)

    runtime = AgentsActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(tmp_path),
        credential_preflight=lambda _sources, _policy: None,
    )

    readiness = runtime.readiness()
    assert readiness.ready is True
    assert readiness.credential_source == "operator_env"


def test_default_runner_blocks_before_actor_when_credentials_missing(tmp_path, monkeypatch):
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    monkeypatch.delenv("RAIL_ACTOR_RUNTIME_LIVE", raising=False)
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runtime = AgentsActorRuntime(project_root=Path("."), policy=load_effective_policy(tmp_path))

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert "credential" in result.structured_output["error"]


def test_live_runner_blocks_invalid_operator_credential_before_actor_work(tmp_path, monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "not-a-real-key")
    monkeypatch.setenv("RAIL_ACTOR_RUNTIME_LIVE", "1")
    called = False

    async def fail_if_called(*_args, **_kwargs):
        nonlocal called
        called = True
        raise AssertionError("SDK runner should not be called when credentials are invalid")

    monkeypatch.setattr("agents.Runner.run", fail_if_called)
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runtime = AgentsActorRuntime(project_root=Path("."), policy=load_effective_policy(tmp_path))

    result = runtime.run(build_invocation(handle, "planner"))

    assert called is False
    assert result.status == "interrupted"
    assert result.blocked_category == "environment"
    assert "invalid credential" in str(result.structured_output["error"])
    assert "not-a-real-key" not in str(result.structured_output["error"])


def test_live_runner_blocks_failed_credential_preflight_before_actor_work(tmp_path, monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test-secret")
    monkeypatch.setenv("RAIL_ACTOR_RUNTIME_LIVE", "1")
    called = False

    async def fail_if_called(*_args, **_kwargs):
        nonlocal called
        called = True
        raise AssertionError("SDK runner should not be called when credential preflight fails")

    monkeypatch.setattr("agents.Runner.run", fail_if_called)
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runtime = AgentsActorRuntime(
        project_root=Path("."),
        policy=load_effective_policy(tmp_path),
        credential_preflight=lambda _sources, _policy: "operator SDK invalid credential is configured",
    )

    result = runtime.run(build_invocation(handle, "planner"))

    assert called is False
    assert result.status == "interrupted"
    assert result.blocked_category == "environment"
    assert "invalid credential" in str(result.structured_output["error"])


def test_live_credential_preflight_uses_policy_timeout(tmp_path, monkeypatch):
    captured: dict[str, object] = {}

    class FakeModels:
        def list(self, *, timeout=None):
            captured["timeout"] = timeout
            return object()

    class FakeOpenAI:
        def __init__(self, *, api_key):
            captured["api_key"] = api_key
            self.models = FakeModels()

    monkeypatch.setattr("openai.OpenAI", FakeOpenAI)
    base_policy = load_effective_policy(tmp_path)
    policy = base_policy.model_copy(
        update={"runtime": base_policy.runtime.model_copy(update={"timeout_seconds": 7})}
    )

    failure = validate_live_sdk_credentials(_credential_sources("sk-test-secret"), policy)

    assert failure is None
    assert captured == {"api_key": "sk-test-secret", "timeout": 7}


def test_agents_runtime_builds_prompt_schema_and_validates_output(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    calls: list[dict[str, object]] = []

    def runner(agent, prompt, *, run_config):
        calls.append({"agent": agent, "prompt": prompt, "run_config": run_config})
        return SDKRunResult(
            final_output={
                "summary": "Plan",
                "likely_files": ["src/rail/api.py"],
                "substeps": ["Do work"],
                "risks": [],
                "acceptance_criteria_refined": ["Pass tests"],
            },
            trace_id="trace-123",
        )

    runtime = AgentsActorRuntime(project_root=Path("."), policy=load_effective_policy(Path(".")), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "succeeded"
    assert result.structured_output["summary"] == "Plan"
    assert calls[0]["agent"].output_type.__name__ == "PlanOutput"
    assert "Run Rail actor planner" in calls[0]["prompt"]
    assert "Exercise agents runtime." in calls[0]["prompt"]
    assert calls[0]["run_config"]["timeout_seconds"] == 180
    assert calls[0]["run_config"]["max_actor_turns"] == 3
    assert calls[0]["run_config"]["approval_policy"] == "never"
    assert (handle.artifact_dir / result.runtime_evidence_ref).read_text(encoding="utf-8")


def test_agents_runtime_maps_sdk_errors_to_interruption(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))

    def runner(_agent, _prompt, *, run_config):
        raise RuntimeError("sdk unavailable")

    runtime = AgentsActorRuntime(project_root=Path("."), policy=load_effective_policy(Path(".")), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert "sdk unavailable" in result.structured_output["error"]


def test_agents_runtime_maps_unexpected_sdk_errors_to_redacted_interruption(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))

    def runner(_agent, _prompt, *, run_config):
        raise Exception("OPENAI_API_KEY=sk-secret-value")

    runtime = AgentsActorRuntime(project_root=Path("."), policy=load_effective_policy(Path(".")), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert "sk-secret-value" not in result.structured_output["error"]
    assert "[REDACTED]" in result.structured_output["error"]


def test_agents_runtime_redacts_secret_events(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))

    def runner(_agent, _prompt, *, run_config):
        return SDKRunResult(
            final_output={
                "decision": "pass",
                "evaluated_input_digest": "sha256:evaluator",
                "findings": [],
                "reason_codes": [],
                "quality_confidence": "high",
            },
            trace_id="sk-secret",
        )

    runtime = AgentsActorRuntime(project_root=Path("."), policy=load_effective_policy(Path(".")), runner=runner)

    result = runtime.run(build_invocation(handle, "evaluator"))

    evidence = (handle.artifact_dir / result.runtime_evidence_ref).read_text(encoding="utf-8")
    assert "sk-secret" not in evidence
    assert "[REDACTED]" in evidence


def test_policy_default_constructs_no_host_tools():
    policy = load_effective_policy(Path("."))

    assert build_sdk_tools(policy) == []


def test_offline_real_sdk_adapter_construction_does_not_require_network():
    runtime = AgentsActorRuntime(project_root=Path("."), policy=load_effective_policy(Path(".")))

    agent = runtime.build_agent("planner")

    assert agent.name == "rail-planner"
    assert agent.output_type.__name__ == "PlanOutput"
    assert agent.tools == []


def test_live_runner_applies_policy_bounds(monkeypatch):
    calls: list[dict[str, object]] = []

    class FinalOutput(BaseModel):
        decision: str = "pass"
        evaluated_input_digest: str = "sha256:evaluator"
        findings: list[str] = []
        reason_codes: list[str] = []
        quality_confidence: str = "high"

    class RunResult:
        final_output = FinalOutput()
        trace_id = "trace-bounds"

    async def fake_run(agent, prompt, *, max_turns, run_config):
        calls.append({"max_turns": max_turns, "workflow_name": run_config.workflow_name})
        return RunResult()

    monkeypatch.setattr("agents.Runner.run", fake_run)

    result = run_agent_live(
        object(),
        "prompt",
        run_config={"timeout_seconds": 3, "max_actor_turns": 2, "approval_policy": "never"},
    )

    assert result.trace_id == "trace-bounds"
    assert calls == [{"max_turns": 2, "workflow_name": "Rail Actor Runtime"}]


def test_agents_runtime_rejects_invalid_final_output(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))

    def runner(_agent, _prompt, *, run_config):
        return SDKRunResult(final_output={"wrong": True}, trace_id="trace-invalid")

    runtime = AgentsActorRuntime(project_root=Path("."), policy=load_effective_policy(Path(".")), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

    assert result.status == "interrupted"
    assert "validation" in result.structured_output["error"]


def test_agents_runtime_rejects_invalid_evaluator_digest_shape(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))

    def runner(_agent, _prompt, *, run_config):
        return SDKRunResult(
            final_output={
                "decision": "pass",
                "evaluated_input_digest": "not-a-digest",
                "findings": [],
                "reason_codes": [],
                "quality_confidence": "high",
            },
            trace_id="trace-invalid-digest",
        )

    runtime = AgentsActorRuntime(project_root=Path("."), policy=load_effective_policy(Path(".")), runner=runner)

    result = runtime.run(build_invocation(handle, "evaluator"))

    assert result.status == "interrupted"
    assert "validation" in result.structured_output["error"]


def _target_repo(tmp_path: Path) -> Path:
    target = tmp_path / "target-repo"
    target.mkdir(parents=True, exist_ok=True)
    return target


def _draft(target: Path) -> dict[str, object]:
    return {
        "project_root": str(target),
        "task_type": "bug_fix",
        "goal": "Exercise agents runtime.",
        "definition_of_done": ["SDK adapter is offline-testable."],
    }


def _credential_sources(value: str) -> list[CredentialSource]:
    return [CredentialSource(category="operator_env", name="OPENAI_API_KEY", value=value)]
