from __future__ import annotations

from pathlib import Path

import rail
from rail.artifacts.projection import project_result


def test_result_projection_reads_artifacts_only(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    rail.supervise(handle)

    projection = project_result(handle)

    assert projection.outcome == "pass"
    assert projection.current_phase == "terminal"
    assert projection.terminal_decision == "pass"
    assert projection.evidence_refs
    assert "src/rail/api.py" in projection.changed_files
    assert projection.residual_risk == "low"
    assert projection.next_step == "complete"


def test_status_and_result_api_project_from_handle(tmp_path):
    handle = rail.start_task(_draft(_target_repo(tmp_path)))
    rail.supervise(handle)

    status = rail.status(handle)
    result = rail.result(handle)

    assert status.current_phase == "terminal"
    assert status.current_actor == "evaluator"
    assert status.next_step == "complete"
    assert result.outcome == "pass"


def _target_repo(tmp_path: Path) -> Path:
    target = tmp_path / "target-repo"
    target.mkdir(parents=True, exist_ok=True)
    return target


def _draft(target: Path) -> dict[str, object]:
    return {
        "project_root": str(target),
        "task_type": "bug_fix",
        "goal": "Project result.",
        "definition_of_done": ["Projection reads artifacts."],
    }
