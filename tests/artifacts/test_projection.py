from __future__ import annotations

from pathlib import Path

import pytest
import yaml

import rail
from rail.artifacts.projection import project_result
from tests.actor_runtime_test_fixtures import scripted_agents_runtime


def test_result_projection_reads_artifacts_only(tmp_path):
    target = _target_repo(tmp_path)
    handle = rail.start_task(_draft(target))
    rail.supervise(handle, runtime=scripted_agents_runtime(target))

    projection = project_result(handle)

    assert projection.outcome == "pass"
    assert projection.outcome_label == "pass"
    assert projection.blocked_category is None
    assert projection.current_phase == "terminal"
    assert projection.terminal_decision == "pass"
    assert projection.reason
    assert projection.evidence_refs
    assert "validation/evidence.yaml" in projection.evidence_refs
    assert "src/rail/api.py" in projection.changed_files
    assert projection.residual_risk == "low"
    assert projection.next_step == "complete"


def test_status_and_result_api_project_from_handle(tmp_path):
    target = _target_repo(tmp_path)
    handle = rail.start_task(_draft(target))
    rail.supervise(handle, runtime=scripted_agents_runtime(target))

    status = rail.status(handle)
    result = rail.result(handle)

    assert status.current_phase == "terminal"
    assert status.current_actor == "evaluator"
    assert status.next_step == "complete"
    assert result.outcome == "pass"


@pytest.mark.parametrize("blocked_category", ["runtime", "validation", "policy", "environment"])
def test_result_projection_projects_blocked_categories_from_artifacts_only(tmp_path, blocked_category):
    target = _target_repo(tmp_path)
    handle = rail.start_task(_draft(target))
    (handle.artifact_dir / "run_status.yaml").write_text(
        yaml.safe_dump(
            {
                "schema_version": "1",
                "artifact_id": handle.artifact_id,
                "status": "blocked",
                "outcome": "blocked",
                "current_actor": "evaluator",
                "blocked_category": blocked_category,
                "reason": f"{blocked_category} blocked reason",
                "visited": ["planner"],
            },
            sort_keys=True,
        ),
        encoding="utf-8",
    )

    projection = rail.result(handle)

    assert projection.outcome == "blocked"
    assert projection.outcome_label == f"{blocked_category}_blocked"
    assert projection.current_phase == "terminal"
    assert projection.terminal_decision == "blocked"
    assert projection.blocked_category == blocked_category
    assert projection.reason == f"{blocked_category} blocked reason"
    assert projection.next_step


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
