from __future__ import annotations

import shutil
from pathlib import Path

import rail
from tests.actor_runtime_test_fixtures import scripted_agents_runtime
from rail.workspace.validation import load_validation_evidence


def test_python_harness_direct_api_smoke_allocates_supervises_and_projects(tmp_path):
    target = tmp_path / "target"
    shutil.copytree(Path("examples/python-target"), target)

    first = rail.start_task(_draft(target, "Fix the sample greeting."))
    rail.supervise(first, runtime=scripted_agents_runtime(target))
    first_result = rail.result(first)
    validation = load_validation_evidence(first.artifact_dir, Path("validation/evidence.yaml"))

    assert first_result.outcome == "pass"
    assert first_result.evidence_refs
    assert validation.command != "policy:validation"
    assert validation.duration_ms >= 0

    before_status_artifacts = set((target / ".harness" / "artifacts").iterdir())
    status = rail.status(first)
    result = rail.result(first)
    after_status_artifacts = set((target / ".harness" / "artifacts").iterdir())

    assert status.current_phase == "terminal"
    assert result.outcome == "pass"
    assert before_status_artifacts == after_status_artifacts

    second = rail.start_task(_draft(target, "Add a separate sample validation."))

    assert second.artifact_id != first.artifact_id
    assert second.artifact_dir != first.artifact_dir


def _draft(target: Path, goal: str) -> dict[str, object]:
    return {
        "project_root": str(target),
        "task_type": "bug_fix",
        "goal": goal,
        "definition_of_done": ["The direct API smoke reaches result projection."],
    }
