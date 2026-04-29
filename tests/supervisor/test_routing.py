from __future__ import annotations

from pathlib import Path

import yaml

import rail
from rail.resources import load_default_asset_yaml
from rail.actor_runtime.runtime import ActorInvocation, ActorResult
from rail.supervisor.graph import SUPERVISOR_GRAPH
from rail.supervisor.router import route_next
from rail.supervisor.state import SupervisorState
from rail.workspace.isolation import tree_digest


def test_initial_supervisor_graph_is_deterministic():
    assert SUPERVISOR_GRAPH == ("planner", "context_builder", "critic", "generator", "executor", "evaluator")


def test_linear_routing_reaches_evaluator():
    state = SupervisorState.created("artifact-1")

    for expected in SUPERVISOR_GRAPH:
        assert state.current_actor == expected
        output = {"decision": "pass"} if expected == "evaluator" else {}
        state = route_next(state, actor_output=output)

    assert state.terminal is True
    assert state.outcome == "pass"


def test_evaluator_revise_routes_back_to_generator_with_budget():
    state = SupervisorState(artifact_id="artifact-1", current_actor="evaluator", revision_budget=1)

    routed = route_next(state, actor_output={"decision": "revise"})

    assert routed.current_actor == "generator"
    assert routed.revision_budget == 0
    assert routed.terminal is False


def test_evaluator_reject_is_terminal_failure():
    state = SupervisorState(artifact_id="artifact-1", current_actor="evaluator", revision_budget=1)

    routed = route_next(state, actor_output={"decision": "reject"})

    assert routed.terminal is True
    assert routed.outcome == "reject"


def test_evaluator_output_without_decision_blocks_instead_of_passing():
    state = SupervisorState(artifact_id="artifact-1", current_actor="evaluator", revision_budget=1)

    routed = route_next(state, actor_output={"error": "sdk failed"})

    assert routed.terminal is True
    assert routed.outcome == "blocked"


def test_supervise_blocks_when_actor_runtime_interrupts(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))

    result = rail.supervise(handle, runtime=InterruptingRuntime())

    assert result.terminal is True
    assert result.outcome == "blocked"
    run_status = yaml.safe_load((handle.artifact_dir / "run_status.yaml").read_text(encoding="utf-8"))
    assert run_status["status"] == "blocked"
    assert run_status["outcome"] == "blocked"


def test_supervise_redacts_runtime_errors_from_terminal_artifacts(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))

    rail.supervise(handle, runtime=SecretInterruptingRuntime())

    run_status = (handle.artifact_dir / "run_status.yaml").read_text(encoding="utf-8")
    terminal_summary = (handle.artifact_dir / "terminal_summary.yaml").read_text(encoding="utf-8")
    assert "sk-secret-value" not in run_status
    assert "sk-secret-value" not in terminal_summary
    assert "[REDACTED]" in run_status
    assert "[REDACTED]" in terminal_summary


def test_supervise_maps_runtime_exceptions_to_blocked_artifact(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))

    result = rail.supervise(handle, runtime=ExplodingRuntime())

    assert result.outcome == "blocked"
    assert (handle.artifact_dir / "run_status.yaml").is_file()
    evidence = (handle.artifact_dir / "runs" / "planner.runtime_evidence.json").read_text(encoding="utf-8")
    assert "sdk exploded" in evidence


def test_default_supervise_does_not_fake_terminal_pass(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))

    result = rail.supervise(handle)

    assert result.terminal is True
    assert result.outcome == "blocked"


def test_default_supervise_blocks_environment_when_runtime_not_ready(tmp_path, monkeypatch):
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    monkeypatch.delenv("RAIL_ACTOR_RUNTIME_LIVE", raising=False)
    handle = rail.start_task(_draft(_target_repo(tmp_path)))

    result = rail.supervise(handle)

    assert result.terminal is True
    assert result.outcome == "blocked"
    run_status = yaml.safe_load((handle.artifact_dir / "run_status.yaml").read_text(encoding="utf-8"))
    assert run_status["current_actor"] == "planner"
    assert run_status["blocked_category"] == "environment"
    projection = rail.result(handle)
    assert projection.current_phase == "terminal"
    assert projection.terminal_decision == "blocked"
    assert projection.blocked_category == "environment"
    assert rail.status(handle).current_actor == "planner"


def test_supervise_records_policy_blocked_when_effective_policy_invalid(tmp_path):
    target = _target_repo(tmp_path)
    policy = load_default_asset_yaml("defaults/supervisor/actor_runtime.yaml")
    policy["workspace"]["network_mode"] = "enabled"
    (target / ".harness" / "supervisor" / "actor_runtime.yaml").write_text(
        yaml.safe_dump(policy, sort_keys=True),
        encoding="utf-8",
    )
    handle = rail.start_task(_draft(target))

    result = rail.supervise(handle, runtime=ExplodingRuntime())

    assert result.outcome == "blocked"
    run_status = yaml.safe_load((handle.artifact_dir / "run_status.yaml").read_text(encoding="utf-8"))
    assert run_status["blocked_category"] == "policy"
    assert "effective policy rejected" in run_status["reason"]
    projection = rail.result(handle)
    assert projection.outcome_label == "policy_blocked"


def test_supervise_policy_block_does_not_persist_policy_file_payload(tmp_path):
    target = _target_repo(tmp_path)
    secret_policy_payload = "secret-policy-payload-should-not-persist"
    secret_file = tmp_path / "secret-policy.txt"
    secret_file.write_text(secret_policy_payload, encoding="utf-8")
    policy_path = target / ".harness" / "supervisor" / "actor_runtime.yaml"
    policy_path.symlink_to(secret_file)
    handle = rail.start_task(_draft(target))

    result = rail.supervise(handle, runtime=ExplodingRuntime())

    assert result.outcome == "blocked"
    run_status = (handle.artifact_dir / "run_status.yaml").read_text(encoding="utf-8")
    terminal_summary = (handle.artifact_dir / "terminal_summary.yaml").read_text(encoding="utf-8")
    assert secret_policy_payload not in run_status
    assert secret_policy_payload not in terminal_summary


def test_supervise_api_updates_artifact_run_status(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))

    result = rail.supervise(handle, runtime=PassingRuntime())

    assert result.outcome == "pass"
    run_status = yaml.safe_load((handle.artifact_dir / "run_status.yaml").read_text(encoding="utf-8"))
    assert run_status["status"] == "terminal"
    assert run_status["outcome"] == "pass"
    assert run_status["current_actor"] == "evaluator"


def test_supervise_applies_generator_patch_bundle_inside_supervision(tmp_path):
    target = _target_repo(tmp_path)
    (target / "app.txt").write_text("old\n", encoding="utf-8")
    handle = rail.start_task(_draft(target))

    result = rail.supervise(handle, runtime=PatchRuntime(target))

    assert result.outcome == "pass"
    assert (target / "app.txt").read_text(encoding="utf-8") == "new\n"


def test_supervise_uses_policy_validation_command_and_blocks_when_missing(tmp_path):
    target = tmp_path / "target-without-validation"
    target.mkdir()
    handle = rail.start_task(_draft(target))

    result = rail.supervise(handle, runtime=PassingRuntime())

    assert result.outcome == "blocked"
    run_status = yaml.safe_load((handle.artifact_dir / "run_status.yaml").read_text(encoding="utf-8"))
    assert run_status["blocked_category"] == "validation"
    assert "validation command" in run_status["reason"]


def test_supervise_routes_evaluator_revise_back_to_generator(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runtime = RevisingRuntime()

    result = rail.supervise(handle, runtime=runtime)

    assert result.outcome == "pass"
    assert runtime.seen == ["planner", "context_builder", "critic", "generator", "executor", "evaluator", "generator", "executor", "evaluator"]


def test_actor_invocation_contains_request_and_prior_outputs(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    runtime = CapturingRuntime()

    rail.supervise(handle, runtime=runtime)

    context_invocation = runtime.invocations["context_builder"]
    evaluator_invocation = runtime.invocations["evaluator"]
    assert context_invocation.input["request"]["goal"] == "Route the supervisor graph."
    assert "planner" in context_invocation.input["prior_outputs"]
    assert "validation/evidence.yaml" in evaluator_invocation.input["evidence_refs"]
    assert "validation_evidence_digest" in evaluator_invocation.input
    assert "evaluator_input_digest" in evaluator_invocation.input


def test_supervise_blocks_evaluator_output_digest_mismatch(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))

    result = rail.supervise(handle, runtime=MismatchedEvaluatorDigestRuntime())

    assert result.outcome == "blocked"
    run_status = yaml.safe_load((handle.artifact_dir / "run_status.yaml").read_text(encoding="utf-8"))
    assert run_status["blocked_category"] == "runtime"
    assert "evaluator input digest" in run_status["reason"]


def _target_repo(tmp_path: Path) -> Path:
    target = tmp_path / "target-repo"
    target.mkdir(parents=True, exist_ok=True)
    _write_validation_policy(target)
    return target


def _draft(target: Path) -> dict[str, object]:
    return {
        "project_root": str(target),
        "task_type": "bug_fix",
        "goal": "Route the supervisor graph.",
        "definition_of_done": ["Supervisor reaches terminal pass."],
    }


class InterruptingRuntime:
    def run(self, invocation: ActorInvocation) -> ActorResult:
        _write_runtime_files(invocation, "interrupted")
        return ActorResult(
            status="interrupted",
            structured_output={"error": "sdk failed"},
            events_ref=Path("runs") / f"{invocation.actor}.events.jsonl",
            runtime_evidence_ref=Path("runs") / f"{invocation.actor}.runtime_evidence.json",
        )


class SecretInterruptingRuntime:
    def run(self, invocation: ActorInvocation) -> ActorResult:
        _write_runtime_files(invocation, "interrupted")
        return ActorResult(
            status="interrupted",
            structured_output={"error": "OPENAI_API_KEY=sk-secret-value"},
            events_ref=Path("runs") / f"{invocation.actor}.events.jsonl",
            runtime_evidence_ref=Path("runs") / f"{invocation.actor}.runtime_evidence.json",
        )


class ExplodingRuntime:
    def run(self, invocation: ActorInvocation) -> ActorResult:
        raise RuntimeError("sdk exploded")


class PassingRuntime:
    def run(self, invocation: ActorInvocation) -> ActorResult:
        from tests.actor_runtime_test_fixtures import fake_actor_output

        output = fake_actor_output(invocation.actor)
        if invocation.actor == "evaluator":
            evaluator_input_digest = invocation.input.get("evaluator_input_digest")
            if isinstance(evaluator_input_digest, str):
                output["evaluated_input_digest"] = evaluator_input_digest
        if invocation.actor == "generator":
            _write_noop_patch_bundle(invocation)
        _write_runtime_files(invocation, "succeeded")
        return ActorResult(
            status="succeeded",
            structured_output=output,
            events_ref=Path("runs") / f"{invocation.actor}.events.jsonl",
            runtime_evidence_ref=Path("runs") / f"{invocation.actor}.runtime_evidence.json",
            patch_bundle_ref=Path("patches/generator.patch.yaml") if invocation.actor == "generator" else None,
        )


class RevisingRuntime(PassingRuntime):
    def __init__(self) -> None:
        self.seen: list[str] = []
        self.revised = False

    def run(self, invocation: ActorInvocation) -> ActorResult:
        self.seen.append(invocation.actor)
        result = super().run(invocation)
        if invocation.actor == "evaluator" and not self.revised:
            self.revised = True
            output = {
                "decision": "revise",
                "evaluated_input_digest": invocation.input["evaluator_input_digest"],
                "findings": ["fix"],
                "reason_codes": ["implementation_gap"],
                "quality_confidence": "medium",
            }
            return result.model_copy(update={"structured_output": output})
        return result


class MismatchedEvaluatorDigestRuntime(PassingRuntime):
    def run(self, invocation: ActorInvocation) -> ActorResult:
        result = super().run(invocation)
        if invocation.actor != "evaluator":
            return result
        output = dict(result.structured_output)
        output["evaluated_input_digest"] = "sha256:wrong-evaluator-input"
        return result.model_copy(update={"structured_output": output})


class CapturingRuntime(PassingRuntime):
    def __init__(self) -> None:
        self.invocations: dict[str, ActorInvocation] = {}

    def run(self, invocation: ActorInvocation) -> ActorResult:
        self.invocations[invocation.actor] = invocation
        return super().run(invocation)


class PatchRuntime(PassingRuntime):
    def __init__(self, target: Path) -> None:
        self.target = target

    def run(self, invocation: ActorInvocation) -> ActorResult:
        result = super().run(invocation)
        if invocation.actor != "generator":
            return result
        output = dict(result.structured_output)
        output["patch_bundle"] = {
            "schema_version": "1",
            "base_tree_digest": tree_digest(self.target),
            "operations": [{"op": "write", "path": "app.txt", "content": "new\n"}],
        }
        return result.model_copy(update={"structured_output": output})


def _write_runtime_files(invocation: ActorInvocation, status: str) -> None:
    runs_dir = invocation.artifact_dir / "runs"
    runs_dir.mkdir(exist_ok=True)
    (runs_dir / f"{invocation.actor}.events.jsonl").write_text(f'{{"status":"{status}"}}\n', encoding="utf-8")
    (runs_dir / f"{invocation.actor}.runtime_evidence.json").write_text(f'{{"status":"{status}"}}', encoding="utf-8")


def _write_noop_patch_bundle(invocation: ActorInvocation) -> None:
    request = yaml.safe_load((invocation.artifact_dir / "request.yaml").read_text(encoding="utf-8"))
    target_root = Path(str(request["project_root"]))
    patch_dir = invocation.artifact_dir / "patches"
    patch_dir.mkdir(exist_ok=True)
    (patch_dir / "generator.patch.yaml").write_text(
        yaml.safe_dump({"schema_version": "1", "base_tree_digest": tree_digest(target_root), "operations": []}),
        encoding="utf-8",
    )


def _write_validation_policy(target: Path) -> None:
    policy = target / ".harness" / "supervisor" / "execution_policy.yaml"
    policy.parent.mkdir(parents=True, exist_ok=True)
    policy.write_text(
        yaml.safe_dump(
            {
                "version": 2,
                "validation": {
                    "commands": [
                        "python -c \"import pathlib; assert pathlib.Path('.').exists()\"",
                    ]
                },
            },
            sort_keys=True,
        ),
        encoding="utf-8",
    )
