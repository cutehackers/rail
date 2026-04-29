from __future__ import annotations

import shutil
from pathlib import Path

import rail


def test_python_harness_direct_api_smoke_allocates_supervises_and_projects(tmp_path):
    target = tmp_path / "target"
    shutil.copytree(Path("examples/python-target"), target)

    first = rail.start_task(_draft(target, "Fix the sample greeting."))
    rail.supervise(first)
    first_result = rail.result(first)

    assert first_result.outcome == "pass"
    assert first_result.evidence_refs

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
