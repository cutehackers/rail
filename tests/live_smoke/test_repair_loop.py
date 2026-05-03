from __future__ import annotations

import json
from pathlib import Path

from rail.actor_runtime.runtime import ActorResult
from rail.live_smoke.models import LiveSmokeActor, LiveSmokeVerdict
from rail.live_smoke.repair_loop import LiveSmokeRepairLoop
from rail.live_smoke.repair_models import RepairLoopStatus
from rail.workspace.isolation import tree_digest


class SequenceRuntime:
    def __init__(self, results: list[ActorResult]) -> None:
        self.results = results
        self.invocations = []

    def run(self, invocation):
        self.invocations.append(invocation)
        result = self.results.pop(0)
        if result.status == "succeeded" and invocation.actor == "generator":
            seed = invocation.input["live_smoke_seed"]
            result = result.model_copy(
                update={
                    "structured_output": {
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
                                    "binary": False,
                                    "executable": False,
                                }
                            ],
                        },
                        "patch_bundle_ref": None,
                    }
                }
            )
        runtime_evidence = {
            "actor": invocation.actor,
            "error": result.structured_output.get("error", ""),
            "policy_violation": {"reason": result.structured_output.get("error", "")},
            "output_schema_ref": f"actor_runtime/schemas/{invocation.actor}.schema.json",
        }
        for evidence_ref in (result.events_ref, result.runtime_evidence_ref):
            evidence_path = invocation.artifact_dir / evidence_ref
            evidence_path.parent.mkdir(parents=True, exist_ok=True)
            evidence_path.write_text(json.dumps(runtime_evidence), encoding="utf-8")
        return result


class AllSuccessRuntime:
    def __init__(self) -> None:
        self.invocations = []

    def run(self, invocation):
        self.invocations.append(invocation)
        output = _success_output_for(invocation)
        result = ActorResult(
            status="succeeded",
            structured_output=output,
            events_ref=Path(f"runs/attempt-0001/{invocation.actor}.events.jsonl"),
            runtime_evidence_ref=Path(f"runs/attempt-0001/{invocation.actor}.runtime_evidence.json"),
        )
        for evidence_ref in (result.events_ref, result.runtime_evidence_ref):
            evidence_path = invocation.artifact_dir / evidence_ref
            evidence_path.parent.mkdir(parents=True, exist_ok=True)
            evidence_path.write_text(json.dumps({"actor": invocation.actor}), encoding="utf-8")
        return result


def _policy_failure(actor: LiveSmokeActor = LiveSmokeActor.GENERATOR) -> ActorResult:
    return ActorResult(
        status="interrupted",
        structured_output={"error": "shell executable is not allowed: python"},
        events_ref=Path(f"runs/attempt-0001/{actor.value}.events.jsonl"),
        runtime_evidence_ref=Path(f"runs/attempt-0001/{actor.value}.runtime_evidence.json"),
        blocked_category="policy",
    )


def _planner_success() -> ActorResult:
    return ActorResult(
        status="succeeded",
        structured_output={"summary": "Plan", "substeps": [], "risks": [], "acceptance_criteria_refined": []},
        events_ref=Path("runs/attempt-0001/planner.events.jsonl"),
        runtime_evidence_ref=Path("runs/attempt-0001/planner.runtime_evidence.json"),
    )


def _generator_success() -> ActorResult:
    return ActorResult(
        status="succeeded",
        structured_output={
            "changed_files": [],
            "patch_summary": [],
            "tests_added_or_updated": [],
            "known_limits": [],
            "patch_bundle_ref": None,
            "patch_bundle": None,
        },
        events_ref=Path("runs/attempt-0001/generator.events.jsonl"),
        runtime_evidence_ref=Path("runs/attempt-0001/generator.runtime_evidence.json"),
    )


def _success_output_for(invocation) -> dict[str, object]:
    if invocation.actor == "planner":
        return {"summary": "Plan", "substeps": [], "risks": [], "acceptance_criteria_refined": []}
    if invocation.actor == "context_builder":
        return {
            "relevant_files": [{"path": "app/service.py", "why": "Fixture service"}],
            "repo_patterns": ["Small Python fixture"],
            "test_patterns": ["Focused service test"],
            "forbidden_changes": ["Do not mutate target directly"],
            "implementation_hints": ["Use a patch bundle"],
        }
    if invocation.actor == "critic":
        return {
            "priority_focus": ["Policy boundary"],
            "missing_requirements": [],
            "risk_hypotheses": [],
            "validation_expectations": ["pytest"],
            "generator_guardrails": ["patch bundle only"],
            "blocked_assumptions": [],
        }
    if invocation.actor == "generator":
        seed = invocation.input["live_smoke_seed"]
        return {
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
                        "binary": False,
                        "executable": False,
                    }
                ],
            },
            "patch_bundle_ref": None,
        }
    if invocation.actor == "executor":
        return {
            "format": "pass",
            "analyze": "pass",
            "tests": {"total": 1, "passed": 1, "failed": 0},
            "failure_details": [],
            "logs": ["pytest passed"],
        }
    if invocation.actor == "evaluator":
        return {
            "decision": "pass",
            "evaluated_input_digest": invocation.input["evaluator_input_digest"],
            "findings": [],
            "reason_codes": [],
            "quality_confidence": "high",
            "next_action": None,
        }
    raise AssertionError(f"unexpected actor {invocation.actor}")


def _repo_root(tmp_path: Path) -> Path:
    repo_root = tmp_path / "repo"
    prompt = repo_root / ".harness" / "actors" / "generator.md"
    prompt.parent.mkdir(parents=True)
    prompt.write_text("You are the Generator actor.\n", encoding="utf-8")
    return repo_root


def test_repair_loop_dry_run_produces_candidate_without_editing_files(tmp_path: Path) -> None:
    runtime = SequenceRuntime([_policy_failure()])
    repo_root = _repo_root(tmp_path)
    before = tree_digest(repo_root)
    loop = LiveSmokeRepairLoop(
        report_root=tmp_path / "reports",
        repo_root=repo_root,
        runtime_factory=lambda _target: runtime,
    )

    report = loop.run_actor(LiveSmokeActor.GENERATOR)

    assert report.status == RepairLoopStatus.CANDIDATE_READY
    assert report.iterations[0].candidate is not None
    assert tree_digest(repo_root) == before


def test_repair_loop_apply_mode_applies_patch_and_reruns_actor(tmp_path: Path) -> None:
    runtime = SequenceRuntime([_policy_failure(), _generator_success()])
    repo_root = _repo_root(tmp_path)
    loop = LiveSmokeRepairLoop(
        report_root=tmp_path / "reports",
        repo_root=repo_root,
        runtime_factory=lambda _target: runtime,
        validation_runner=lambda _candidate, _repo_root: True,
    )

    report = loop.run_actor(LiveSmokeActor.GENERATOR, apply=True, max_iterations=2)

    assert report.status == RepairLoopStatus.REPAIRED
    assert len(runtime.invocations) == 2
    assert report.iterations[0].applied_patch_digest is not None
    assert "do not probe unavailable tools" in (repo_root / ".harness" / "actors" / "generator.md").read_text(
        encoding="utf-8"
    )


def test_repair_loop_keeps_provider_failure_unrepairable(tmp_path: Path) -> None:
    runtime = SequenceRuntime(
        [
            ActorResult(
                status="interrupted",
                structured_output={"error": "provider timeout"},
                events_ref=Path("runs/attempt-0001/generator.events.jsonl"),
                runtime_evidence_ref=Path("runs/attempt-0001/generator.runtime_evidence.json"),
                blocked_category="environment",
            )
        ]
    )
    loop = LiveSmokeRepairLoop(
        report_root=tmp_path / "reports",
        repo_root=_repo_root(tmp_path),
        runtime_factory=lambda _target: runtime,
    )

    report = loop.run_actor(LiveSmokeActor.GENERATOR, apply=True)

    assert report.status == RepairLoopStatus.UNREPAIRABLE
    assert report.iterations[0].candidate is None


def test_repair_loop_reports_budget_exhaustion(tmp_path: Path) -> None:
    runtime = SequenceRuntime([_policy_failure()])
    loop = LiveSmokeRepairLoop(
        report_root=tmp_path / "reports",
        repo_root=_repo_root(tmp_path),
        runtime_factory=lambda _target: runtime,
        validation_runner=lambda _candidate, _repo_root: True,
    )

    report = loop.run_actor(LiveSmokeActor.GENERATOR, apply=True, max_iterations=1)

    assert report.status == RepairLoopStatus.BUDGET_EXHAUSTED
    assert report.iterations[0].candidate is not None


def test_repair_loop_run_all_passes_when_all_actors_pass(tmp_path: Path) -> None:
    runtime = AllSuccessRuntime()
    loop = LiveSmokeRepairLoop(
        report_root=tmp_path / "reports",
        repo_root=_repo_root(tmp_path),
        runtime_factory=lambda _target: runtime,
    )

    report = loop.run_all()

    assert report.status == RepairLoopStatus.PASSED
    assert [iteration.actor for iteration in report.iterations] == list(LiveSmokeActor)
    assert all(iteration.candidate is None for iteration in report.iterations)
    assert all(
        json.loads((iteration.report_path).read_text(encoding="utf-8"))["verdict"] == LiveSmokeVerdict.PASSED
        for iteration in report.iterations
    )
