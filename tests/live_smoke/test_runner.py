from __future__ import annotations

import json
from pathlib import Path

from rail.actor_runtime.runtime import ActorResult
from rail.live_smoke.models import (
    LiveSmokeActor,
    LiveSmokeVerdict,
    OwningSurface,
    SymptomClass,
)
from rail.live_smoke.runner import LiveSmokeRunner


class FakeRuntime:
    def __init__(self, result: ActorResult) -> None:
        self.result = result
        self.invocation_actors: list[str] = []

    def run(self, invocation):
        self.invocation_actors.append(invocation.actor)
        return self.result


class WritingRuntime:
    def __init__(self, result: ActorResult) -> None:
        self.result = result
        self.invocations = []

    def run(self, invocation):
        self.invocations.append(invocation)
        for evidence_ref in (self.result.events_ref, self.result.runtime_evidence_ref):
            evidence_path = invocation.artifact_dir / evidence_ref
            evidence_path.parent.mkdir(parents=True, exist_ok=True)
            evidence_path.write_text("{}", encoding="utf-8")
        return self.result


class RaisingRuntime:
    def run(self, invocation):
        raise TimeoutError("provider timed out")


class SeedAwareGeneratorRuntime:
    def __init__(self) -> None:
        self.invocations = []

    def run(self, invocation):
        self.invocations.append(invocation)
        seed = invocation.input["live_smoke_seed"]
        output = {
            "changed_files": ["app/service.py"],
            "patch_summary": ["Updated fixture service."],
            "tests_added_or_updated": [],
            "known_limits": [],
            "patch_bundle": {
                "schema_version": "1",
                "base_tree_digest": seed["fixture_digest"],
                "operations": [
                    {
                        "op": "write",
                        "path": "app/service.py",
                        "content": "from __future__ import annotations\n",
                    },
                ],
            },
        }
        for evidence_ref in (
            Path("runs/attempt-0001/generator.events.jsonl"),
            Path("runs/attempt-0001/generator.runtime_evidence.json"),
        ):
            evidence_path = invocation.artifact_dir / evidence_ref
            evidence_path.parent.mkdir(parents=True, exist_ok=True)
            evidence_path.write_text("{}", encoding="utf-8")
        return ActorResult(
            status="succeeded",
            structured_output=output,
            events_ref=Path("runs/attempt-0001/generator.events.jsonl"),
            runtime_evidence_ref=Path("runs/attempt-0001/generator.runtime_evidence.json"),
        )


class SeedAwareEvaluatorRuntime:
    def __init__(self) -> None:
        self.invocations = []

    def run(self, invocation):
        self.invocations.append(invocation)
        output = {
            "decision": "pass",
            "evaluated_input_digest": invocation.input["evaluator_input_digest"],
            "findings": [],
            "reason_codes": [],
            "quality_confidence": "high",
        }
        for evidence_ref in (
            Path("runs/attempt-0001/evaluator.events.jsonl"),
            Path("runs/attempt-0001/evaluator.runtime_evidence.json"),
        ):
            evidence_path = invocation.artifact_dir / evidence_ref
            evidence_path.parent.mkdir(parents=True, exist_ok=True)
            evidence_path.write_text("{}", encoding="utf-8")
        return ActorResult(
            status="succeeded",
            structured_output=output,
            events_ref=Path("runs/attempt-0001/evaluator.events.jsonl"),
            runtime_evidence_ref=Path("runs/attempt-0001/evaluator.runtime_evidence.json"),
        )


def test_runner_passes_planner_smoke(tmp_path: Path) -> None:
    runtime = WritingRuntime(
        ActorResult(
            status="succeeded",
            structured_output={"summary": "Plan", "substeps": [], "risks": [], "acceptance_criteria_refined": []},
            events_ref=Path("runs/attempt-0001/planner.events.jsonl"),
            runtime_evidence_ref=Path("runs/attempt-0001/planner.runtime_evidence.json"),
        )
    )
    runner = LiveSmokeRunner(report_root=tmp_path / "reports", runtime_factory=lambda _target: runtime)

    report = runner.run_actor(LiveSmokeActor.PLANNER)

    assert report.verdict == LiveSmokeVerdict.PASSED
    assert report.symptom_class is None
    assert report.seed_schema_version == "1"
    assert report.seed_digest is not None
    assert report.synthetic_seed is True


def test_runner_fails_when_runtime_omits_evidence_files(tmp_path: Path) -> None:
    runtime = FakeRuntime(
        ActorResult(
            status="succeeded",
            structured_output={"summary": "Plan", "substeps": [], "risks": [], "acceptance_criteria_refined": []},
            events_ref=Path("runs/attempt-0001/planner.events.jsonl"),
            runtime_evidence_ref=Path("runs/attempt-0001/planner.runtime_evidence.json"),
        )
    )
    runner = LiveSmokeRunner(report_root=tmp_path / "reports", runtime_factory=lambda _target: runtime)

    report = runner.run_actor(LiveSmokeActor.PLANNER)

    payload = json.loads((report.report_dir / "live_smoke_report.json").read_text(encoding="utf-8"))
    assert report.verdict == LiveSmokeVerdict.FAILED
    assert report.symptom_class == SymptomClass.EVIDENCE_WRITER_FAILURE
    assert report.owning_surface == OwningSurface.PROVIDER
    assert payload["symptom_class"] == "evidence_writer_failure"


def test_runner_writes_json_report_with_resolvable_artifact_evidence(tmp_path: Path) -> None:
    runtime = WritingRuntime(
        ActorResult(
            status="succeeded",
            structured_output={"summary": "Plan", "substeps": [], "risks": [], "acceptance_criteria_refined": []},
            events_ref=Path("runs/attempt-0001/planner.events.jsonl"),
            runtime_evidence_ref=Path("runs/attempt-0001/planner.runtime_evidence.json"),
        )
    )
    runner = LiveSmokeRunner(report_root=tmp_path / "reports", runtime_factory=lambda _target: runtime)

    report = runner.run_actor(LiveSmokeActor.PLANNER)

    payload = json.loads((report.report_dir / "live_smoke_report.json").read_text(encoding="utf-8"))
    assert payload["actor"] == "planner"
    assert payload["verdict"] == "passed"
    assert payload["artifact_id"] == report.artifact_id
    assert Path(payload["artifact_dir"]) == report.artifact_dir
    assert report.artifact_id is not None
    assert report.artifact_dir is not None
    assert all((report.artifact_dir / evidence_ref).is_file() for evidence_ref in report.evidence_refs)
    assert payload["seed_schema_version"] == "1"
    assert payload["seed_digest"] == report.seed_digest
    assert payload["synthetic_seed"] is True


def test_runner_passes_live_smoke_seed_to_invocation(tmp_path: Path) -> None:
    runtime = WritingRuntime(
        ActorResult(
            status="succeeded",
            structured_output={"summary": "Plan", "substeps": [], "risks": [], "acceptance_criteria_refined": []},
            events_ref=Path("runs/attempt-0001/planner.events.jsonl"),
            runtime_evidence_ref=Path("runs/attempt-0001/planner.runtime_evidence.json"),
        )
    )
    runner = LiveSmokeRunner(report_root=tmp_path / "reports", runtime_factory=lambda _target: runtime)

    report = runner.run_actor(LiveSmokeActor.PLANNER)

    seed = runtime.invocations[0].input["live_smoke_seed"]
    assert seed["actor"] == "planner"
    assert seed["seed_digest"] == report.seed_digest
    assert seed["synthetic"] is True
    actor_input = runtime.invocations[0].input
    runtime_contract = actor_input["live_smoke_runtime_contract"]
    assert actor_input["request"]["project_root"] == "."
    assert runtime_contract["shell_working_directory"] == "."
    assert runtime_contract["allowed_shell_executables"] == [
        "cat",
        "find",
        "head",
        "ls",
        "pwd",
        "rg",
        "sed",
        "stat",
        "tail",
        "test",
        "wc",
    ]
    assert "ruff" in runtime_contract["forbidden_shell_executables"]
    assert "Do not probe unavailable tools" in runtime_contract["tool_availability_probe"]


def test_runner_passes_generator_with_seeded_patch_bundle(tmp_path: Path) -> None:
    runtime = SeedAwareGeneratorRuntime()
    runner = LiveSmokeRunner(report_root=tmp_path / "reports", runtime_factory=lambda _target: runtime)

    report = runner.run_actor(LiveSmokeActor.GENERATOR)

    assert report.verdict == LiveSmokeVerdict.PASSED
    assert report.symptom_class is None
    assert runtime.invocations[0].input["live_smoke_seed"]["expected_patch_paths"] == ["app/service.py"]
    assert runtime.invocations[0].input["request"]["project_root"] == "."


def test_runner_binds_evaluator_input_digest(tmp_path: Path) -> None:
    runtime = SeedAwareEvaluatorRuntime()
    runner = LiveSmokeRunner(report_root=tmp_path / "reports", runtime_factory=lambda _target: runtime)

    report = runner.run_actor(LiveSmokeActor.EVALUATOR)

    assert report.verdict == LiveSmokeVerdict.PASSED
    assert report.symptom_class is None
    assert runtime.invocations[0].input["validation_evidence_digest"] == "sha256:live-smoke-synthetic-validation"
    assert runtime.invocations[0].input["evaluator_input_digest"].startswith("sha256:")


def test_runner_reports_context_builder_policy_failure(tmp_path: Path) -> None:
    runtime = WritingRuntime(
        ActorResult(
            status="interrupted",
            structured_output={"error": "shell executable is not allowed: grep"},
            events_ref=Path("runs/attempt-0001/context_builder.events.jsonl"),
            runtime_evidence_ref=Path("runs/attempt-0001/context_builder.runtime_evidence.json"),
            blocked_category="policy",
        )
    )
    runner = LiveSmokeRunner(report_root=tmp_path / "reports", runtime_factory=lambda _target: runtime)

    report = runner.run_actor(LiveSmokeActor.CONTEXT_BUILDER)

    assert report.verdict == LiveSmokeVerdict.FAILED
    assert report.symptom_class == SymptomClass.POLICY_VIOLATION
    assert report.repair_proposal is not None


def test_runner_reports_behavior_smoke_failure(tmp_path: Path) -> None:
    runtime = WritingRuntime(
        ActorResult(
            status="succeeded",
            structured_output={"summary": "Plan"},
            events_ref=Path("runs/attempt-0001/planner.events.jsonl"),
            runtime_evidence_ref=Path("runs/attempt-0001/planner.runtime_evidence.json"),
        )
    )
    runner = LiveSmokeRunner(report_root=tmp_path / "reports", runtime_factory=lambda _target: runtime)

    report = runner.run_actor(LiveSmokeActor.PLANNER)

    payload = json.loads((report.report_dir / "live_smoke_report.json").read_text(encoding="utf-8"))
    assert report.verdict == LiveSmokeVerdict.FAILED
    assert report.symptom_class == SymptomClass.BEHAVIOR_SMOKE_FAILURE
    assert report.owning_surface == OwningSurface.ACTOR_PROMPT
    assert payload["symptom_class"] == "behavior_smoke_failure"
    assert payload["owning_surface"] == "actor_prompt"


def test_runner_reports_runtime_exception_failure(tmp_path: Path) -> None:
    runner = LiveSmokeRunner(report_root=tmp_path / "reports", runtime_factory=lambda _target: RaisingRuntime())

    report = runner.run_actor(LiveSmokeActor.PLANNER)

    payload = json.loads((report.report_dir / "live_smoke_report.json").read_text(encoding="utf-8"))
    assert report.verdict == LiveSmokeVerdict.FAILED
    assert report.symptom_class == SymptomClass.PROVIDER_TRANSIENT_FAILURE
    assert report.owning_surface == OwningSurface.PROVIDER
    assert report.artifact_id is not None
    assert report.artifact_dir is not None
    assert report.evidence_refs == []
    assert payload["symptom_class"] == "provider_transient_failure"


def test_runner_reports_fixture_setup_exception(
    tmp_path: Path,
    monkeypatch,
) -> None:
    from rail.live_smoke import runner as runner_module

    def fail_copy(*_args: object, **_kwargs: object) -> object:
        raise OSError("fixture unavailable")

    monkeypatch.setattr(runner_module, "copy_fixture_target", fail_copy)
    runner = LiveSmokeRunner(report_root=tmp_path / "reports", runtime_factory=lambda _target: RaisingRuntime())

    report = runner.run_actor(LiveSmokeActor.PLANNER)

    payload = json.loads((report.report_dir / "live_smoke_report.json").read_text(encoding="utf-8"))
    assert report.verdict == LiveSmokeVerdict.FAILED
    assert report.symptom_class == SymptomClass.FIXTURE_PREP_FAILURE
    assert report.owning_surface == OwningSurface.FIXTURE
    assert report.artifact_id is None
    assert report.artifact_dir is None
    assert payload["symptom_class"] == "fixture_prep_failure"


def test_run_all_returns_reports_for_every_actor_after_runtime_exceptions(tmp_path: Path) -> None:
    runner = LiveSmokeRunner(report_root=tmp_path / "reports", runtime_factory=lambda _target: RaisingRuntime())

    reports = runner.run_all()

    assert [report.actor for report in reports] == [
        LiveSmokeActor.PLANNER,
        LiveSmokeActor.CONTEXT_BUILDER,
        LiveSmokeActor.CRITIC,
        LiveSmokeActor.GENERATOR,
        LiveSmokeActor.EXECUTOR,
        LiveSmokeActor.EVALUATOR,
    ]
    assert [report.symptom_class for report in reports] == [
        SymptomClass.PROVIDER_TRANSIENT_FAILURE,
        SymptomClass.PROVIDER_TRANSIENT_FAILURE,
        SymptomClass.PROVIDER_TRANSIENT_FAILURE,
        SymptomClass.PROVIDER_TRANSIENT_FAILURE,
        SymptomClass.PROVIDER_TRANSIENT_FAILURE,
        SymptomClass.PROVIDER_TRANSIENT_FAILURE,
    ]
