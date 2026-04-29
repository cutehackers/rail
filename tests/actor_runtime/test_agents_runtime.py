from __future__ import annotations

from pathlib import Path

import rail
from rail.actor_runtime.agents import AgentsActorRuntime, SDKRunResult, build_sdk_tools
from rail.actor_runtime.runtime import build_invocation
from rail.policy import load_effective_policy


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

    runtime = AgentsActorRuntime(project_root=Path("."), policy=load_effective_policy(tmp_path))

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
    assert calls[0]["run_config"]["timeout_seconds"] == 180
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


def test_agents_runtime_redacts_secret_events(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))

    def runner(_agent, _prompt, *, run_config):
        return SDKRunResult(final_output={"decision": "pass", "findings": [], "reason_codes": [], "quality_confidence": "high"}, trace_id="sk-secret")

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


def test_agents_runtime_rejects_invalid_final_output(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))

    def runner(_agent, _prompt, *, run_config):
        return SDKRunResult(final_output={"wrong": True}, trace_id="trace-invalid")

    runtime = AgentsActorRuntime(project_root=Path("."), policy=load_effective_policy(Path(".")), runner=runner)

    result = runtime.run(build_invocation(handle, "planner"))

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
