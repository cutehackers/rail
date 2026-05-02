from __future__ import annotations

from pathlib import Path

from rail.actor_runtime.runtime import ActorResult
from rail.live_smoke.models import LiveSmokeActor, LiveSmokeVerdict, SymptomClass
from rail.live_smoke.runner import LiveSmokeRunner


class FakeRuntime:
    def __init__(self, result: ActorResult) -> None:
        self.result = result
        self.invocation_actors: list[str] = []

    def run(self, invocation):
        self.invocation_actors.append(invocation.actor)
        return self.result


def test_runner_passes_planner_smoke(tmp_path: Path) -> None:
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

    assert report.verdict == LiveSmokeVerdict.PASSED
    assert report.symptom_class is None
    assert runtime.invocation_actors == ["planner"]


def test_runner_reports_context_builder_policy_failure(tmp_path: Path) -> None:
    runtime = FakeRuntime(
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
